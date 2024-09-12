package policies

import (
	"context"
	"net/http"
	"time"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
)

func EventIPRateLimiter(tokensPerInterval int, interval time.Duration, maxTokens int) func(ctx context.Context, _ *nostr.Event) (reject bool, msg string) {
	rl := startRateLimitSystem[string](tokensPerInterval, interval, maxTokens)

	return func(ctx context.Context, _ *nostr.Event) (reject bool, msg string) {
		ip := khatru.GetIP(ctx)
		if ip == "" {
			return false, ""
		}
		return rl(ip), "rate-limited: slow down, please"
	}
}

func EventPubKeyRateLimiter(tokensPerInterval int, interval time.Duration, maxTokens int) func(ctx context.Context, _ *nostr.Event) (reject bool, msg string) {
	rl := startRateLimitSystem[string](tokensPerInterval, interval, maxTokens)

	return func(ctx context.Context, evt *nostr.Event) (reject bool, msg string) {
		return rl(evt.PubKey), "rate-limited: slow down, please"
	}
}

func ConnectionRateLimiter(tokensPerInterval int, interval time.Duration, maxTokens int) func(r *http.Request) bool {
	rl := startRateLimitSystem[string](tokensPerInterval, interval, maxTokens)

	return func(r *http.Request) bool {
		return rl(khatru.GetIPFromRequest(r))
	}
}

func FilterIPRateLimiter(tokensPerInterval int, interval time.Duration, maxTokens int) func(ctx context.Context, _ nostr.Filter) (reject bool, msg string) {
	rl := startRateLimitSystem[string](tokensPerInterval, interval, maxTokens)

	return func(ctx context.Context, _ nostr.Filter) (reject bool, msg string) {
		return rl(khatru.GetIP(ctx)), "rate-limited: there is a bug in the client, no one should be making so many requests"
	}
}
