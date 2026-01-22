package khatru

import (
	"context"

	"github.com/nbd-wtf/go-nostr"
)

const (
	wsKey = iota
	subscriptionIdKey
	nip86HeaderAuthKey
	internalCallKey
)

func RequestAuth(ctx context.Context) {
	ws := GetConnection(ctx)
	ws.authLock.Lock()
	if ws.Authed == nil {
		ws.Authed = make(chan struct{})
	}
	ws.authLock.Unlock()
	ws.WriteJSON(nostr.AuthEnvelope{Challenge: &ws.Challenge})
}

func GetConnection(ctx context.Context) *WebSocket {
	wsi := ctx.Value(wsKey)
	if wsi != nil {
		return wsi.(*WebSocket)
	}
	return nil
}

// GetAuthed returns the first authenticated pubkey (for backward compatibility).
// Returns the nip86 header auth pubkey if present, otherwise the first pubkey from the WebSocket auth set.
func GetAuthed(ctx context.Context) string {
	if nip86Auth := ctx.Value(nip86HeaderAuthKey); nip86Auth != nil {
		return nip86Auth.(string)
	}
	if conn := GetConnection(ctx); conn != nil && conn.AuthedPublicKeys != nil {
		for pubkey := range conn.AuthedPublicKeys {
			return pubkey
		}
	}
	return ""
}

// IsAuthed checks if a specific pubkey is authenticated on this connection.
func IsAuthed(ctx context.Context, pubkey string) bool {
	if nip86Auth := ctx.Value(nip86HeaderAuthKey); nip86Auth != nil {
		return nip86Auth.(string) == pubkey
	}
	if conn := GetConnection(ctx); conn != nil && conn.AuthedPublicKeys != nil {
		_, exists := conn.AuthedPublicKeys[pubkey]
		return exists
	}
	return false
}

// GetAuthedKeys returns all authenticated pubkeys for this connection.
func GetAuthedKeys(ctx context.Context) []string {
	if nip86Auth := ctx.Value(nip86HeaderAuthKey); nip86Auth != nil {
		return []string{nip86Auth.(string)}
	}
	if conn := GetConnection(ctx); conn != nil && conn.AuthedPublicKeys != nil {
		keys := make([]string, 0, len(conn.AuthedPublicKeys))
		for pubkey := range conn.AuthedPublicKeys {
			keys = append(keys, pubkey)
		}
		return keys
	}
	return nil
}

// IsInternalCall returns true when a call to QueryEvents, for example, is being made because of a deletion
// or expiration request.
func IsInternalCall(ctx context.Context) bool {
	return ctx.Value(internalCallKey) != nil
}

func GetIP(ctx context.Context) string {
	conn := GetConnection(ctx)
	if conn == nil {
		return ""
	}

	return GetIPFromRequest(conn.Request)
}

func GetSubscriptionID(ctx context.Context) string {
	return ctx.Value(subscriptionIdKey).(string)
}
