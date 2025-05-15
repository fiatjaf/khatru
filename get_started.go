package khatru

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/rs/cors"
)

func (rl *Relay) Router() *http.ServeMux {
	return rl.serveMux
}

func (rl *Relay) SetRouter(mux *http.ServeMux) {
	rl.serveMux = mux
}

// Start creates an http server and starts listening on given host and port.
func (rl *Relay) Start(host string, port int, started ...chan bool) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	rl.Addr = ln.Addr().String()
	rl.httpServer = &http.Server{
		Handler:      cors.Default().Handler(rl),
		Addr:         addr,
		WriteTimeout: 2 * time.Second,
		ReadTimeout:  2 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// notify caller that we're starting
	for _, started := range started {
		close(started)
	}

	if err := rl.httpServer.Serve(ln); err == http.ErrServerClosed {
		return nil
	} else if err != nil {
		return err
	} else {
		return nil
	}
}

// Shutdown sends a websocket close control message to all connected clients.
func (rl *Relay) Shutdown(ctx context.Context) {
	rl.httpServer.Shutdown(ctx)
	rl.clientsMutex.Lock()
	defer rl.clientsMutex.Unlock()
	for ws := range rl.clients {
		ws.conn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(time.Second))
		ws.cancel()
		ws.conn.Close()
	}
	clear(rl.clients)
	rl.listeners = rl.listeners[:0]
}
