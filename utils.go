package khatru

import (
	"context"

	"github.com/nbd-wtf/go-nostr"
)

const (
	wsKey = iota
	subscriptionIdKey
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
	conn := GetConnection(ctx)
	if conn != nil {
		return conn.AuthedPublicKey
	}
	return ""
}

func GetIP(ctx context.Context) string {
	return GetIPFromRequest(GetConnection(ctx).Request)
}

func GetSubscriptionID(ctx context.Context) string {
	return ctx.Value(subscriptionIdKey).(string)
}

func GetOpenSubscriptions(ctx context.Context) []nostr.Filter {
	if subs, ok := listeners.Load(GetConnection(ctx)); ok {
		res := make([]nostr.Filter, 0, listeners.Size()*2)
		subs.Range(func(_ string, sub *Listener) bool {
			res = append(res, sub.filters...)
			return true
		})
		return res
	}
	return nil
}
