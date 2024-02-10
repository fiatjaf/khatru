package khatru

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	} else {
		rl.serveMux.ServeHTTP(w, r)
	}
}

func challenge(conn *websocket.Conn) *WebSocket {
	// NIP-42 challenge
	challenge := make([]byte, 8)
	rand.Read(challenge)

	return &WebSocket{
		conn:      conn,
		Challenge: hex.EncodeToString(challenge),
	}
}

func (rl *Relay) doEvent(ctx context.Context, ws *WebSocket, env *nostr.EventEnvelope) {
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

	var ok bool
	var writeErr error
	if env.Event.Kind == 5 {
		// this always returns "blocked: " whenever it returns an error
		writeErr = rl.handleDeleteRequest(ctx, &env.Event)
	} else {
		// this will also always return a prefixed reason
		writeErr = rl.AddEvent(ctx, &env.Event)
	}

	var reason string
	if writeErr == nil {
		ok = true
		for _, ovw := range rl.OverwriteResponseEvent {
			ovw(ctx, &env.Event)
		}
		notifyListeners(&env.Event)
	} else {
		reason = writeErr.Error()
		if strings.HasPrefix(reason, "auth-required:") {
			RequestAuth(ctx)
		}
	}
	ws.WriteJSON(nostr.OKEnvelope{EventID: env.Event.ID, OK: ok, Reason: reason})
}

func (rl *Relay) doCount(ctx context.Context, ws *WebSocket, env *nostr.CountEnvelope) {
	if rl.CountEvents == nil {
		ws.WriteJSON(nostr.ClosedEnvelope{SubscriptionID: env.SubscriptionID, Reason: "unsupported: this relay does not support NIP-45"})
		return
	}
	var total int64
	for _, filter := range env.Filters {
		total += rl.handleCountRequest(ctx, ws, filter)
	}
	ws.WriteJSON(nostr.CountEnvelope{SubscriptionID: env.SubscriptionID, Count: &total})
}

func (rl *Relay) doReq(ctx context.Context, ws *WebSocket, env *nostr.ReqEnvelope) {
	eose := sync.WaitGroup{}
	eose.Add(len(env.Filters))

	// a context just for the "stored events" request handler
	reqCtx, cancelReqCtx := context.WithCancelCause(ctx)

	// expose subscription id in the context
	reqCtx = context.WithValue(reqCtx, subscriptionIdKey, env.SubscriptionID)

	// handle each filter separately -- dispatching events as they're loaded from databases
	for _, filter := range env.Filters {
		err := rl.handleRequest(reqCtx, env.SubscriptionID, &eose, ws, filter)
		if err != nil {
			// fail everything if any filter is rejected
			reason := err.Error()
			if strings.HasPrefix(reason, "auth-required:") {
				RequestAuth(ctx)
			}
			ws.WriteJSON(nostr.ClosedEnvelope{SubscriptionID: env.SubscriptionID, Reason: reason})
			cancelReqCtx(errors.New("filter rejected"))
			return
		}
	}

	go func() {
		// when all events have been loaded from databases and dispatched
		// we can cancel the context and fire the EOSE message
		eose.Wait()
		cancelReqCtx(nil)
		ws.WriteJSON(nostr.EOSEEnvelope(env.SubscriptionID))
	}()

	setListener(env.SubscriptionID, ws, env.Filters, cancelReqCtx)
}

func (rl *Relay) doClose(ctx context.Context, ws *WebSocket, env *nostr.CloseEnvelope) {
	removeListenerId(ws, string(*env))
}

func (rl *Relay) doAuth(ctx context.Context, ws *WebSocket, env *nostr.AuthEnvelope) {
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

func (rl *Relay) handleMessage(ctx context.Context, ws *WebSocket, message []byte) {
	envelope := nostr.ParseMessage(message)
	if envelope == nil {
		// stop silently
		return
	}

	switch env := envelope.(type) {
	case *nostr.EventEnvelope:
		rl.doEvent(ctx, ws, env)
	case *nostr.CountEnvelope:
		rl.doCount(ctx, ws, env)
	case *nostr.ReqEnvelope:
		rl.doReq(ctx, ws, env)
	case *nostr.CloseEnvelope:
		rl.doClose(ctx, ws, env)
	case *nostr.AuthEnvelope:
		rl.doAuth(ctx, ws, env)
	}
}

func (rl *Relay) HandleWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := rl.upgrader.Upgrade(w, r, nil)
	if err != nil {
		rl.Log.Printf("failed to upgrade websocket: %v\n", err)
		return
	}
	rl.clients.Store(conn, struct{}{})
	ticker := time.NewTicker(rl.PingPeriod)

	ws := challenge(conn)

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
		if _, ok := rl.clients.Load(conn); ok {
			conn.Close()
			rl.clients.Delete(conn)
			removeListener(ws)
		}
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
				ws.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(rl.WriteWait))
				continue
			}

			go rl.handleMessage(ctx, ws, message)
		}
	}()

	go func() {
		defer kill()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := ws.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(rl.WriteWait))
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

func (rl *Relay) HandleNIP11(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/nostr+json")

	info := *rl.Info
	for _, ovw := range rl.OverwriteRelayInformation {
		info = ovw(r.Context(), r, info)
	}

	json.NewEncoder(w).Encode(info)
}
