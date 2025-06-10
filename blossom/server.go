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

	StoreBlob     []func(ctx context.Context, sha256 string, body []byte) error
	LoadBlob      []func(ctx context.Context, sha256 string) (io.ReadSeeker, error)
	DeleteBlob    []func(ctx context.Context, sha256 string) error
	ReceiveReport []func(ctx context.Context, reportEvt *nostr.Event) error
	RedirectGet   []func(ctx context.Context, sha256 string, fileExtension string) (url string, code int, err error)

	RejectUpload []func(ctx context.Context, auth *nostr.Event, size int, ext string) (bool, string, int)
	RejectGet    []func(ctx context.Context, auth *nostr.Event, sha256 string) (bool, string, int)
	RejectList   []func(ctx context.Context, auth *nostr.Event, pubkey string) (bool, string, int)
	RejectDelete []func(ctx context.Context, auth *nostr.Event, sha256 string) (bool, string, int)
}

// ServerOption represents a functional option for configuring a BlossomServer
type ServerOption func(*BlossomServer)

// New creates a new BlossomServer with the given relay and service URL
// Optional configuration can be provided via functional options
func New(rl *khatru.Relay, serviceURL string) *BlossomServer {
	bs := &BlossomServer{
		ServiceURL: serviceURL,
	}

	base := rl.Router()
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" {
			if r.Method == "PUT" {
				bs.handleUpload(w, r)
				return
			} else if r.Method == "HEAD" {
				bs.handleUploadCheck(w, r)
				return
			}
		}
		if r.URL.Path == "/media" {
			bs.handleMedia(w, r)
			return
		}
		if r.URL.Path == "/mirror" && r.Method == "PUT" {
			bs.handleMirror(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/list/") && r.Method == "GET" {
			bs.handleList(w, r)
			return
		}

		if (len(r.URL.Path) == 65 || strings.Index(r.URL.Path, ".") == 65) && strings.Index(r.URL.Path[1:], "/") == -1 {
			if r.Method == "HEAD" {
				bs.handleHasBlob(w, r)
				return
			} else if r.Method == "GET" {
				bs.handleGetBlob(w, r)
				return
			} else if r.Method == "DELETE" {
				bs.handleDelete(w, r)
				return
			}
		}

		if r.URL.Path == "/report" {
			if r.Method == "PUT" {
				bs.handleReport(w, r)
				return
			}
		}

		base.ServeHTTP(w, r)
	})

	rl.SetRouter(mux)

	return bs
}
