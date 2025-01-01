package khatru

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bep/debounce"
	"github.com/fasthttp/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip42"
	"github.com/nbd-wtf/go-nostr/nip45"
	"github.com/nbd-wtf/go-nostr/nip45/hyperloglog"
	"github.com/nbd-wtf/go-nostr/nip77"
	"github.com/nbd-wtf/go-nostr/nip77/negentropy"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/rs/cors"
)

// ServeHTTP implements http.Handler interface.
func (rl *Relay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowedHeaders: []string{"Authorization", "*"},
		MaxAge:         86400,
	})

	if r.Header.Get("Upgrade") == "websocket" {
		rl.HandleWebsocket(w, r)
	} else if r.Header.Get("Accept") == "application/nostr+json" {
		corsMiddleware.Handler(http.HandlerFunc(rl.HandleNIP11)).ServeHTTP(w, r)
	} else if r.Header.Get("Content-Type") == "application/nostr+json+rpc" {
		corsMiddleware.Handler(http.HandlerFunc(rl.HandleNIP86)).ServeHTTP(w, r)
	} else {
		corsMiddleware.Handler(rl.serveMux).ServeHTTP(w, r)
	}
}

func (rl *Relay) HandleWebsocket(w http.ResponseWriter, r *http.Request) {
	for _, reject := range rl.RejectConnection {
		if reject(r) {
			w.WriteHeader(429) // Too many requests
			return
		}
	}

	conn, err := rl.upgrader.Upgrade(w, r, nil)
	if err != nil {
		rl.Log.Printf("failed to upgrade websocket: %v\n", err)
		return
	}

	ticker := time.NewTicker(rl.PingPeriod)

	// NIP-42 challenge
	challenge := make([]byte, 8)
	rand.Read(challenge)

	ws := &WebSocket{
		conn:               conn,
		Request:            r,
		Challenge:          hex.EncodeToString(challenge),
		negentropySessions: xsync.NewMapOf[string, *NegentropySession](),
	}
	ws.Context, ws.cancel = context.WithCancel(context.Background())

	rl.clientsMutex.Lock()
	rl.clients[ws] = make([]listenerSpec, 0, 2)
	rl.clientsMutex.Unlock()

	ctx, cancel := context.WithCancel(
		context.WithValue(
			context.Background(),
			wsKey, ws,
		),
	)

	kill := func() {
		for _, ondisconnect := range rl.OnDisconnect {
			ondisconnect(ctx)
		}

		ticker.Stop()
		cancel()
		ws.cancel()
		ws.conn.Close()

		rl.removeClientAndListeners(ws)
	}

	go func() {
		defer kill()

		ws.conn.SetReadLimit(rl.MaxMessageSize)
		ws.conn.SetReadDeadline(time.Now().Add(rl.PongWait))
		ws.conn.SetPongHandler(func(string) error {
			ws.conn.SetReadDeadline(time.Now().Add(rl.PongWait))
			return nil
		})

		for _, onconnect := range rl.OnConnect {
			onconnect(ctx)
		}

		for {
			typ, message, err := ws.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(
					err,
					websocket.CloseNormalClosure,    // 1000
					websocket.CloseGoingAway,        // 1001
					websocket.CloseNoStatusReceived, // 1005
					websocket.CloseAbnormalClosure,  // 1006
					4537,                            // some client seems to send many of these
				) {
					rl.Log.Printf("unexpected close error from %s: %v\n", r.Header.Get("X-Forwarded-For"), err)
				}
				ws.cancel()
				return
			}

			if typ == websocket.PingMessage {
				ws.WriteMessage(websocket.PongMessage, nil)
				continue
			}

			go func(message []byte) {
				envelope := nostr.ParseMessage(message)
				if envelope == nil {
					if !rl.Negentropy {
						// stop silently
						return
					}
					envelope = nip77.ParseNegMessage(message)
					if envelope == nil {
						return
					}
				}

				switch env := envelope.(type) {
				case *nostr.EventEnvelope:
					// check id
					if !env.Event.CheckID() {
						ws.WriteJSON(nostr.OKEnvelope{EventID: env.Event.ID, OK: false, Reason: "invalid: id is computed incorrectly"})
						return
					}

					// check signature
					if ok, err := env.Event.CheckSignature(); err != nil {
						ws.WriteJSON(nostr.OKEnvelope{EventID: env.Event.ID, OK: false, Reason: "error: failed to verify signature"})
						return
					} else if !ok {
						ws.WriteJSON(nostr.OKEnvelope{EventID: env.Event.ID, OK: false, Reason: "invalid: signature is invalid"})
						return
					}

					// check NIP-70 protected
					for _, v := range env.Event.Tags {
						if len(v) == 1 && v[0] == "-" {
							msg := "must be published by event author"
							authed := GetAuthed(ctx)
							if authed == "" {
								RequestAuth(ctx)
								ws.WriteJSON(nostr.OKEnvelope{
									EventID: env.Event.ID,
									OK:      false,
									Reason:  "auth-required: " + msg,
								})
								return
							}
							if authed != env.Event.PubKey {
								ws.WriteJSON(nostr.OKEnvelope{
									EventID: env.Event.ID,
									OK:      false,
									Reason:  "blocked: " + msg,
								})
								return
							}
						}
					}

					srl := rl
					if rl.getSubRelayFromEvent != nil {
						srl = rl.getSubRelayFromEvent(&env.Event)
					}

					var ok bool
					var writeErr error
					var skipBroadcast bool

					if env.Event.Kind == 5 {
						// this always returns "blocked: " whenever it returns an error
						writeErr = srl.handleDeleteRequest(ctx, &env.Event)
					} else {
						// this will also always return a prefixed reason
						skipBroadcast, writeErr = srl.AddEvent(ctx, &env.Event)
					}

					var reason string
					if writeErr == nil {
						ok = true
						for _, ovw := range srl.OverwriteResponseEvent {
							ovw(ctx, &env.Event)
						}
						if !skipBroadcast {
							srl.notifyListeners(&env.Event)
						}
					} else {
						reason = writeErr.Error()
						if strings.HasPrefix(reason, "auth-required:") {
							RequestAuth(ctx)
						}
					}
					ws.WriteJSON(nostr.OKEnvelope{EventID: env.Event.ID, OK: ok, Reason: reason})
				case *nostr.CountEnvelope:
					if rl.CountEvents == nil && rl.CountEventsHLL == nil {
						ws.WriteJSON(nostr.ClosedEnvelope{SubscriptionID: env.SubscriptionID, Reason: "unsupported: this relay does not support NIP-45"})
						return
					}

					var total int64
					var hll *hyperloglog.HyperLogLog
					uneligibleForHLL := false

					for _, filter := range env.Filters {
						srl := rl
						if rl.getSubRelayFromFilter != nil {
							srl = rl.getSubRelayFromFilter(filter)
						}

						if offset := nip45.HyperLogLogEventPubkeyOffsetForFilter(filter); offset != -1 && !uneligibleForHLL {
							partial, phll := srl.handleCountRequestWithHLL(ctx, ws, filter, offset)
							if phll != nil {
								if hll == nil {
									// in the first iteration (which should be the only case of the times)
									// we optimize slightly by assigning instead of merging
									hll = phll
								} else {
									hll.Merge(phll)
								}
							} else {
								// if any of the filters is uneligible then we will discard previous HLL results
								// and refuse to do HLL at all anymore for this query
								uneligibleForHLL = true
								hll = nil
							}
							total += partial
						} else {
							total += srl.handleCountRequest(ctx, ws, filter)
						}
					}

					resp := nostr.CountEnvelope{
						SubscriptionID: env.SubscriptionID,
						Count:          &total,
					}
					if hll != nil {
						resp.HyperLogLog = hll.GetRegisters()
					}

					ws.WriteJSON(resp)

				case *nostr.ReqEnvelope:
					eose := sync.WaitGroup{}
					eose.Add(len(env.Filters))

					// a context just for the "stored events" request handler
					reqCtx, cancelReqCtx := context.WithCancelCause(ctx)

					// expose subscription id in the context
					reqCtx = context.WithValue(reqCtx, subscriptionIdKey, env.SubscriptionID)

					// handle each filter separately -- dispatching events as they're loaded from databases
					for _, filter := range env.Filters {
						srl := rl
						if rl.getSubRelayFromFilter != nil {
							srl = rl.getSubRelayFromFilter(filter)
						}
						err := srl.handleRequest(reqCtx, env.SubscriptionID, &eose, ws, filter)
						if err != nil {
							// fail everything if any filter is rejected
							reason := err.Error()
							if strings.HasPrefix(reason, "auth-required:") {
								RequestAuth(ctx)
							}
							ws.WriteJSON(nostr.ClosedEnvelope{SubscriptionID: env.SubscriptionID, Reason: reason})
							cancelReqCtx(errors.New("filter rejected"))
							return
						} else {
							rl.addListener(ws, env.SubscriptionID, srl, filter, cancelReqCtx)
						}
					}

					go func() {
						// when all events have been loaded from databases and dispatched
						// we can cancel the context and fire the EOSE message
						eose.Wait()
						cancelReqCtx(nil)
						ws.WriteJSON(nostr.EOSEEnvelope(env.SubscriptionID))
					}()
				case *nostr.CloseEnvelope:
					id := string(*env)
					rl.removeListenerId(ws, id)
				case *nostr.AuthEnvelope:
					wsBaseUrl := strings.Replace(rl.getBaseURL(r), "http", "ws", 1)
					if pubkey, ok := nip42.ValidateAuthEvent(&env.Event, ws.Challenge, wsBaseUrl); ok {
						ws.AuthedPublicKey = pubkey
						ws.authLock.Lock()
						if ws.Authed != nil {
							close(ws.Authed)
							ws.Authed = nil
						}
						ws.authLock.Unlock()
						ws.WriteJSON(nostr.OKEnvelope{EventID: env.Event.ID, OK: true})
					} else {
						ws.WriteJSON(nostr.OKEnvelope{EventID: env.Event.ID, OK: false, Reason: "error: failed to authenticate"})
					}
				case *nip77.OpenEnvelope:
					srl := rl
					if rl.getSubRelayFromFilter != nil {
						srl = rl.getSubRelayFromFilter(env.Filter)
						if !srl.Negentropy {
							// ignore
							return
						}
					}
					vec, err := srl.startNegentropySession(ctx, env.Filter)
					if err != nil {
						// fail everything if any filter is rejected
						reason := err.Error()
						if strings.HasPrefix(reason, "auth-required:") {
							RequestAuth(ctx)
						}
						ws.WriteJSON(nip77.ErrorEnvelope{SubscriptionID: env.SubscriptionID, Reason: reason})
						return
					}

					// reconcile to get the next message and return it
					neg := negentropy.New(vec, 1024*1024)
					out, err := neg.Reconcile(env.Message)
					if err != nil {
						ws.WriteJSON(nip77.ErrorEnvelope{SubscriptionID: env.SubscriptionID, Reason: err.Error()})
						return
					}
					ws.WriteJSON(nip77.MessageEnvelope{SubscriptionID: env.SubscriptionID, Message: out})

					// if the message is not empty that means we'll probably have more reconciliation sessions, so store this
					if out != "" {
						deb := debounce.New(time.Second * 7)
						negSession := &NegentropySession{
							neg: neg,
							postponeClose: func() {
								deb(func() {
									ws.negentropySessions.Delete(env.SubscriptionID)
								})
							},
						}
						negSession.postponeClose()

						ws.negentropySessions.Store(env.SubscriptionID, negSession)
					}
				case *nip77.MessageEnvelope:
					negSession, ok := ws.negentropySessions.Load(env.SubscriptionID)
					if !ok {
						// bad luck, your request was destroyed
						ws.WriteJSON(nip77.ErrorEnvelope{SubscriptionID: env.SubscriptionID, Reason: "CLOSED"})
						return
					}
					// reconcile to get the next message and return it
					out, err := negSession.neg.Reconcile(env.Message)
					if err != nil {
						ws.WriteJSON(nip77.ErrorEnvelope{SubscriptionID: env.SubscriptionID, Reason: err.Error()})
						ws.negentropySessions.Delete(env.SubscriptionID)
						return
					}
					ws.WriteJSON(nip77.MessageEnvelope{SubscriptionID: env.SubscriptionID, Message: out})

					// if there is more reconciliation to do, postpone this
					if out != "" {
						negSession.postponeClose()
					} else {
						// otherwise we can just close it
						ws.negentropySessions.Delete(env.SubscriptionID)
					}
				case *nip77.CloseEnvelope:
					ws.negentropySessions.Delete(env.SubscriptionID)
				}
			}(message)
		}
	}()

	go func() {
		defer kill()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := ws.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					if !strings.HasSuffix(err.Error(), "use of closed network connection") {
						rl.Log.Printf("error writing ping: %v; closing websocket\n", err)
					}
					return
				}
			}
		}
	}()
}
