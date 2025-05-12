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

// WithRedirectURL configures a redirect URL for the RedirectGet function
func WithRedirectURL(urlTemplate string, statusCode int) ServerOption {
	return func(bs *BlossomServer) {
		redirectFn := redirectGet(urlTemplate, statusCode)
		bs.RedirectGet = append(bs.RedirectGet, redirectFn)
	}
}

// New creates a new BlossomServer with the given relay and service URL
// Optional configuration can be provided via functional options
func New(rl *khatru.Relay, serviceURL string, opts ...ServerOption) *BlossomServer {
	bs := &BlossomServer{
		ServiceURL: serviceURL,
	}

	// Apply any provided options
	for _, opt := range opts {
		opt(bs)
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

// redirectGet returns a function that redirects to a specified URL template with the given status code.
// The URL template can include {sha256} and/or {extension} placeholders that will be replaced
// with the actual values. If neither placeholder is present, {sha256}.{extension} will be
// appended to the URL with proper forward slash handling.
func redirectGet(urlTemplate string, statusCode int) func(context.Context, string, string) (url string, code int, err error) {
	return func(ctx context.Context, sha256 string, extension string) (string, int, error) {
		finalURL := urlTemplate

		// Replace placeholders if they exist
		hasSHA256Placeholder := strings.Contains(finalURL, "{sha256}")
		hasExtensionPlaceholder := strings.Contains(finalURL, "{extension}")

		if hasSHA256Placeholder {
			finalURL = strings.Replace(finalURL, "{sha256}", sha256, -1)
		}

		if hasExtensionPlaceholder {
			finalURL = strings.Replace(finalURL, "{extension}", extension, -1)
		}

		// If neither placeholder is present, append sha256.extension
		if !hasSHA256Placeholder && !hasExtensionPlaceholder {
			// Ensure URL ends with a forward slash
			if !strings.HasSuffix(finalURL, "/") {
				finalURL += "/"
			}

			finalURL += sha256
			if extension != "" {
				finalURL += "." + extension
			}
		}

		return finalURL, statusCode, nil
	}
}
