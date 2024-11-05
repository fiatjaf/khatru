package blossom

import (
	"context"
	"io"
	"net/http"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
	"github.com/rs/cors"
)

type BlossomServer struct {
	ServiceURL string
	Store      BlobIndex

	StoreBlob  []func(ctx context.Context, sha256 string, body []byte) error
	LoadBlob   []func(ctx context.Context, sha256 string) (io.Reader, error)
	DeleteBlob []func(ctx context.Context, sha256 string) error

	RejectUpload []func(ctx context.Context, auth *nostr.Event, size int, ext string) (bool, string, int)
	RejectGet    []func(ctx context.Context, auth *nostr.Event, sha256 string) (bool, string, int)
	RejectList   []func(ctx context.Context, auth *nostr.Event, pubkey string) (bool, string, int)
	RejectDelete []func(ctx context.Context, auth *nostr.Event, sha256 string) (bool, string, int)
}

func New(rl *khatru.Relay, serviceURL string) *BlossomServer {
	bs := &BlossomServer{
		ServiceURL: serviceURL,
	}

	base := rl.Router()

	blossomApi := http.NewServeMux()
	blossomApi.HandleFunc("PUT /upload", bs.handleUpload)
	blossomApi.HandleFunc("HEAD /upload", bs.handleUploadCheck)
	blossomApi.HandleFunc("GET /list/{pubkey}", bs.handleList)
	blossomApi.HandleFunc("HEAD /{sha256}", bs.handleHasBlob)
	blossomApi.HandleFunc("GET /{sha256}", bs.handleGetBlob)
	blossomApi.HandleFunc("DELETE /{sha256}", bs.handleDelete)
	blossomApi.Handle("/", base) // forwards to relay

	bud01CorsMux := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "PUT", "DELETE"},
		AllowedHeaders: []string{"Authorization", "*"},
		MaxAge:         86400,
	})

	wrappedBlossomApi := bud01CorsMux.Handler(blossomApi)

	combinedMux := http.NewServeMux()
	combinedMux.Handle("/", wrappedBlossomApi)

	rl.SetRouter(combinedMux)

	return bs
}
