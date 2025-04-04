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

func GetAuthed(ctx context.Context) string {
	if conn := GetConnection(ctx); conn != nil {
		return conn.AuthedPublicKey
	}
	if nip86Auth := ctx.Value(nip86HeaderAuthKey); nip86Auth != nil {
		return nip86Auth.(string)
	}
	return ""
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
