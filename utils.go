package khatru

import (
	"context"

	"github.com/nbd-wtf/go-nostr"
)

const (
	wsKey = iota
	subscriptionIdKey
	nip86HeaderAuthKey
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
