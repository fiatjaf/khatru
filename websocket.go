package khatru

import (
	"context"
	"net/http"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/nbd-wtf/go-nostr"
)

type WebSocket struct {
	conn  *websocket.Conn
	mutex sync.Mutex

	// original request
	Request *http.Request

	// this Context will be canceled whenever the connection is closed from the client side or server-side.
	Context context.Context
	cancel  context.CancelFunc

	// nip42
	Challenge       string
	AuthedPublicKey string
	Authed          chan struct{}

	authLock sync.Mutex
}

func (ws *WebSocket) WriteJSON(any any) error {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()
	return ws.conn.WriteJSON(any)
}

func (ws *WebSocket) WriteMessage(t int, b []byte) error {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()
	return ws.conn.WriteMessage(t, b)
}

func (ws *WebSocket) SendEvent(subscriptionId string, event nostr.Event) error {
	return ws.WriteJSON(nostr.EventEnvelope{
		SubscriptionID: &subscriptionId,
		Event:          event,
	})
}
