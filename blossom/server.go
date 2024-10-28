package blossom

import (
	"context"
	"net/http"
	"strings"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
)

type BlossomServer struct {
	ServiceURL string
	Store      BlobIndex

	StoreBlob  []func(ctx context.Context, sha256 string, body []byte) error
	LoadBlob   []func(ctx context.Context, sha256 string) ([]byte, error)
	DeleteBlob []func(ctx context.Context, sha256 string) error

	RejectUpload []func(ctx context.Context, auth *nostr.Event, ext string) (bool, string)
	RejectGet    []func(ctx context.Context, auth *nostr.Event, sha256 string) (bool, string)
	RejectList   []func(ctx context.Context, auth *nostr.Event, pubkey string) (bool, string)
	RejectDelete []func(ctx context.Context, auth *nostr.Event, sha256 string) (bool, string)
}

func New(rl *khatru.Relay, serviceURL string) *BlossomServer {
	bs := &BlossomServer{
		ServiceURL: serviceURL,
	}

	base := rl.Router()
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" && r.Method == "PUT" {
			setCors(w)
			bs.handleUpload(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/list/") && r.Method == "GET" {
			setCors(w)
			bs.handleList(w, r)
			return
		}

		if len(strings.SplitN(r.URL.Path, ".", 2)[0]) == 65 {
			if r.Method == "HEAD" {
				setCors(w)
				bs.handleHasBlob(w, r)
				return
			} else if r.Method == "GET" {
				setCors(w)
				bs.handleGetBlob(w, r)
				return
			} else if r.Method == "DELETE" {
				setCors(w)
				bs.handleDelete(w, r)
				return
			}
		}

		base.ServeHTTP(w, r)
	})

	rl.SetRouter(mux)

	return bs
}
