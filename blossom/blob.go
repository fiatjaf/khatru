package blossom

import "github.com/nbd-wtf/go-nostr"

type Blob struct {
	URL      string          `json:"url"`
	SHA256   string          `json:"sha256"`
	Size     int             `json:"size"`
	Type     string          `json:"type"`
	Uploaded nostr.Timestamp `json:"uploaded"`
}
