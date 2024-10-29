package blossom

import (
	"context"

	"github.com/nbd-wtf/go-nostr"
)

type BlobDescriptor struct {
	URL      string          `json:"url"`
	SHA256   string          `json:"sha256"`
	Size     int             `json:"size"`
	Type     string          `json:"type"`
	Uploaded nostr.Timestamp `json:"uploaded"`

	Owner string `json:"-"`
}

type BlobIndex interface {
	Keep(ctx context.Context, blob BlobDescriptor, pubkey string) error
	List(ctx context.Context, pubkey string) (chan BlobDescriptor, error)
	Get(ctx context.Context, sha256 string) (*BlobDescriptor, error)
	Delete(ctx context.Context, sha256 string, pubkey string) error
}

var _ BlobIndex = (*EventStoreBlobIndexWrapper)(nil)
