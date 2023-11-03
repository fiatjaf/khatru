package khatru

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip11"
	"github.com/puzpuzpuz/xsync/v2"
)

func NewRelay() *Relay {
	return &Relay{
		Log: log.New(os.Stderr, "[khatru-relay] ", log.LstdFlags),

		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},

		clients:  xsync.NewTypedMapOf[*websocket.Conn, struct{}](pointerHasher[websocket.Conn]),
		serveMux: &http.ServeMux{},

		WriteWait:      10 * time.Second,
		PongWait:       60 * time.Second,
		PingPeriod:     30 * time.Second,
		MaxMessageSize: 512000,
	}
}

type Relay struct {
	Name        string
	Description string
	PubKey      string
	Contact     string
	ServiceURL  string // required for nip-42
	IconURL     string

	RejectEvent              []func(ctx context.Context, event *nostr.Event) (reject bool, msg string)
	RejectFilter             []func(ctx context.Context, filter nostr.Filter) (reject bool, msg string)
	RejectCountFilter        []func(ctx context.Context, filter nostr.Filter) (reject bool, msg string)
	OverwriteDeletionOutcome []func(ctx context.Context, target *nostr.Event, deletion *nostr.Event) (acceptDeletion bool, msg string)
	StoreEvent               []func(ctx context.Context, event *nostr.Event) error
	DeleteEvent              []func(ctx context.Context, event *nostr.Event) error
	QueryEvents              []func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error)
	CountEvents              []func(ctx context.Context, filter nostr.Filter) (int64, error)
	EditInformation          []func(ctx context.Context, info *nip11.RelayInformationDocument)
	OnAuth                   []func(ctx context.Context, pubkey string)
	OnConnect                []func(ctx context.Context)
	OnEventSaved             []func(ctx context.Context, event *nostr.Event)

	// Default logger, as set by NewServer, is a stdlib logger prefixed with "[khatru-relay] ",
	// outputting to stderr.
	Log *log.Logger

	// for establishing websockets
	upgrader websocket.Upgrader

	// keep a connection reference to all connected clients for Server.Shutdown
	clients *xsync.MapOf[*websocket.Conn, struct{}]

	// in case you call Server.Start
	Addr       string
	serveMux   *http.ServeMux
	httpServer *http.Server

	// websocket options
	WriteWait      time.Duration // Time allowed to write a message to the peer.
	PongWait       time.Duration // Time allowed to read the next pong message from the peer.
	PingPeriod     time.Duration // Send pings to peer with this period. Must be less than pongWait.
	MaxMessageSize int64         // Maximum message size allowed from peer.
}

func (rl *Relay) RequestAuth(ctx context.Context) {
	ws := GetConnection(ctx)
	ws.WriteJSON(nostr.AuthEnvelope{Challenge: &ws.Challenge})
}
