package khatru

import (
	"sync"

	"github.com/fasthttp/websocket"
)

type WebSocket struct {
	conn  *websocket.Conn
	mutex sync.Mutex

	// nip42
	Challenge      string
	Authed         string
	WaitingForAuth chan struct{}
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
