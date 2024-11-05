package blossom

import (
	"context"
	"io"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
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

	mux := rl.Router()

	mux.HandleFunc("PUT /upload", bs.handleUpload)
	mux.HandleFunc("HEAD /upload", bs.handleUploadCheck)
	mux.HandleFunc("GET /list/{pubkey}", bs.handleList)
	mux.HandleFunc("HEAD /{sha256}", bs.handleHasBlob)
	mux.HandleFunc("GET /{sha256}", bs.handleGetBlob)
	mux.HandleFunc("DELETE /{sha256}", bs.handleDelete)

	return bs
}
