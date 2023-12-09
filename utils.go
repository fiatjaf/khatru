package khatru

import (
	"context"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sebest/xff"
)

func GetConnection(ctx context.Context) *WebSocket {
	return ctx.Value(WS_KEY).(*WebSocket)
}

func GetAuthed(ctx context.Context) string {
	return GetConnection(ctx).AuthedPublicKey
}

func GetIP(ctx context.Context) string {
	return xff.GetRemoteAddr(GetConnection(ctx).Request)
}

func GetOpenSubscriptions(ctx context.Context) []nostr.Filter {
	if listeners, ok := listeners.Load(GetConnection(ctx)); ok {
		res := make([]nostr.Filter, 0, listeners.Size()*2)
		listeners.Range(func(_ string, listener *Listener) bool {
			res = append(res, listener.filters...)
			return true
		})
		return res
	}
	return nil
}
