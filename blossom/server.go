package blossom

import (
	"context"
	"io"
	"net/http"
	"strings"

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

	base := rl.Router()
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			setCors(w)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.URL.Path == "/upload" {
			if r.Method == "PUT" {
				setCors(w)
				bs.handleUpload(w, r)
				return
			} else if r.Method == "HEAD" {
				setCors(w)
				bs.handleUploadCheck(w, r)
				return
			}
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
