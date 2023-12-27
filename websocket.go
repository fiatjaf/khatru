package khatru

import (
	"net/http"
	"sync"

	"github.com/fasthttp/websocket"
)

type WebSocket struct {
	conn  *websocket.Conn
	mutex sync.Mutex

	// original request
	Request *http.Request

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
