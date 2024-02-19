package khatru

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip11"
	"github.com/puzpuzpuz/xsync/v3"
)

type Relay struct {
	ServiceURL string

	RejectEvent               []func(ctx context.Context, event *nostr.Event) (reject bool, msg string)
	RejectFilter              []func(ctx context.Context, filter nostr.Filter) (reject bool, msg string)
	RejectCountFilter         []func(ctx context.Context, filter nostr.Filter) (reject bool, msg string)
	OverwriteDeletionOutcome  []func(ctx context.Context, target *nostr.Event, deletion *nostr.Event) (acceptDeletion bool, msg string)
	OverwriteResponseEvent    []func(ctx context.Context, event *nostr.Event)
	OverwriteFilter           []func(ctx context.Context, filter *nostr.Filter)
	OverwriteCountFilter      []func(ctx context.Context, filter *nostr.Filter)
	OverwriteRelayInformation []func(ctx context.Context, r *http.Request, info nip11.RelayInformationDocument) nip11.RelayInformationDocument
	StoreEvent                []func(ctx context.Context, event *nostr.Event) error
	DeleteEvent               []func(ctx context.Context, event *nostr.Event) error
	QueryEvents               []func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error)
	CountEvents               []func(ctx context.Context, filter nostr.Filter) (int64, error)
	OnConnect                 []func(ctx context.Context)
	OnDisconnect              []func(ctx context.Context)
	OnEventSaved              []func(ctx context.Context, event *nostr.Event)
	OnEphemeralEvent          []func(ctx context.Context, event *nostr.Event)

	// editing info will affect the responses to the NIP-11 endpoint
	Info *nip11.RelayInformationDocument

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

	// this context is used for all things inside the relay
	Context context.Context
	cancel  context.CancelCauseFunc
}

func NewRelay() *Relay {
	return NewRelayWithContext(context.Background())
}

func NewRelayWithContext(ctx context.Context) *Relay {
	ctx, cancel := context.WithCancelCause(ctx)

	rl := &Relay{
		Log: log.New(os.Stderr, "[khatru-relay] ", log.LstdFlags),

		Info: &nip11.RelayInformationDocument{
			Software:      "https://github.com/fiatjaf/khatru",
			Version:       "n/a",
			SupportedNIPs: []int{1, 11, 70},
		},

		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},

		clients:  xsync.NewMapOf[*websocket.Conn, struct{}](),
		serveMux: &http.ServeMux{},

		WriteWait:      10 * time.Second,
		PongWait:       60 * time.Second,
		PingPeriod:     30 * time.Second,
		MaxMessageSize: 512000,

		Context: ctx,
		cancel:  cancel,
	}

	return rl
}

func (rl *Relay) Close() {
	rl.cancel(fmt.Errorf("Close called"))
}
