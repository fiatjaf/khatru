package khatru

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip42"
	"github.com/rs/cors"
)

// ServeHTTP implements http.Handler interface.
func (rl *Relay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if rl.ServiceURL == "" {
		rl.ServiceURL = getServiceBaseURL(r)
	}

	if r.Header.Get("Upgrade") == "websocket" {
		rl.HandleWebsocket(w, r)
	} else if r.Header.Get("Accept") == "application/nostr+json" {
		cors.AllowAll().Handler(http.HandlerFunc(rl.HandleNIP11)).ServeHTTP(w, r)
	} else if r.Header.Get("Content-Type") == "application/nostr+json+rpc" {
		cors.AllowAll().Handler(http.HandlerFunc(rl.HandleNIP86)).ServeHTTP(w, r)
	} else {
		rl.serveMux.ServeHTTP(w, r)
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
		conn:      conn,
		Request:   r,
		Challenge: hex.EncodeToString(challenge),
	}

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
		conn.Close()

		rl.clientsMutex.Lock()
		defer rl.clientsMutex.Unlock()
		if specs, ok := rl.clients[ws]; ok {
			// swap delete listeners and delete client
			for s, spec := range specs {
				// no need to cancel contexts since they inherit from the main connection context
				// just delete the listeners
				srl := spec.subrelay
				srl.listeners[spec.index] = srl.listeners[len(srl.listeners)-1]
				specs[s] = specs[len(specs)-1]
				srl.listeners = srl.listeners[0:len(srl.listeners)]
			}
		}
		delete(rl.clients, ws)
	}

	go func() {
		defer kill()

		conn.SetReadLimit(rl.MaxMessageSize)
		conn.SetReadDeadline(time.Now().Add(rl.PongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(rl.PongWait))
			return nil
		})

		for _, onconnect := range rl.OnConnect {
			onconnect(ctx)
		}

		for {
			typ, message, err := conn.ReadMessage()
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
				return
			}

			if typ == websocket.PingMessage {
				ws.WriteMessage(websocket.PongMessage, nil)
				continue
			}

			go func(message []byte) {
				envelope := nostr.ParseMessage(message)
				if envelope == nil {
					// stop silently
					return
				}

				switch env := envelope.(type) {
				case *nostr.EventEnvelope:
					// check id
					hash := sha256.Sum256(env.Event.Serialize())
					id := hex.EncodeToString(hash[:])
					if id != env.Event.ID {
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
							srl.notifyListeners(&env.Event, nil)
						}
					} else {
						reason = writeErr.Error()
						if strings.HasPrefix(reason, "auth-required:") {
							RequestAuth(ctx)
						}
					}
					ws.WriteJSON(nostr.OKEnvelope{EventID: env.Event.ID, OK: ok, Reason: reason})
				case *nostr.CountEnvelope:
					if rl.CountEvents == nil {
						ws.WriteJSON(nostr.ClosedEnvelope{SubscriptionID: env.SubscriptionID, Reason: "unsupported: this relay does not support NIP-45"})
						return
					}

					var total int64
					for _, filter := range env.Filters {
						srl := rl
						if rl.getSubRelayFromFilter != nil {
							srl = rl.getSubRelayFromFilter(filter)
						}
						total += srl.handleCountRequest(ctx, ws, filter)
					}
					ws.WriteJSON(nostr.CountEnvelope{SubscriptionID: env.SubscriptionID, Count: &total})
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
					wsBaseUrl := strings.Replace(rl.ServiceURL, "http", "ws", 1)
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
