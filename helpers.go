package khatru

import (
	"hash/maphash"
	"regexp"
	"strings"
	"unsafe"

	"github.com/nbd-wtf/go-nostr"
)

const (
	AUTH_CONTEXT_KEY = iota
	WS_KEY
)

var nip20prefixmatcher = regexp.MustCompile(`^\w+: `)

func pointerHasher[V any](_ maphash.Seed, k *V) uint64 {
	return uint64(uintptr(unsafe.Pointer(k)))
}

func isOlder(previous, next *nostr.Event) bool {
	return previous.CreatedAt < next.CreatedAt ||
		(previous.CreatedAt == next.CreatedAt && previous.ID > next.ID)
}

func isAuthRequired(msg string) bool {
	idx := strings.IndexByte(msg, ':')
	return msg[0:idx] == "auth-required"
}
