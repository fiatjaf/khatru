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
