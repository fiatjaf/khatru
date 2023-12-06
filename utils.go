package khatru

import (
	"context"
)

func GetConnection(ctx context.Context) *WebSocket {
	return ctx.Value(WS_KEY).(*WebSocket)
}

func GetAuthed(ctx context.Context) string {
	authedPubkey := ctx.Value(AUTH_CONTEXT_KEY)
	if authedPubkey == nil {
		return ""
	}
	return authedPubkey.(string)
}
