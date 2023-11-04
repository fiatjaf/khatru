package plugins

import (
	"context"

	"github.com/nbd-wtf/go-nostr"
)

// PreventTooManyIndexableTags returns a function that can be used as a RejectFilter that will reject
// events with more indexable (single-character) tags than the specified number.
func PreventTooManyIndexableTags(max int) func(context.Context, *nostr.Event) (bool, string) {
	return func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		ntags := 0
		for _, tag := range event.Tags {
			if len(tag) > 0 && len(tag[0]) == 1 {
				ntags++
			}
		}
		if ntags > max {
			return true, "too many indexable tags"
		}
		return false, ""
	}
}

// RestrictToSpecifiedKinds returns a function that can be used as a RejectFilter that will reject
// any events with kinds different than the specified ones.
func RestrictToSpecifiedKinds(kinds ...uint16) func(context.Context, *nostr.Event) (bool, string) {
	max := 0
	min := 0
	allowed := make(map[uint16]struct{}, len(kinds))
	for _, kind := range kinds {
		allowed[kind] = struct{}{}
		if int(kind) > max {
			max = int(kind)
		}
		if int(kind) < min {
			min = int(kind)
		}
	}

	return func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		// these are cheap and very questionable optimizations, but they exist for a reason:
		// we would have to ensure that the kind number is within the bounds of a uint16 anyway
		if event.Kind > max {
			return true, "event kind not allowed"
		}
		if event.Kind < min {
			return true, "event kind not allowed"
		}

		// hopefully this map of uint16s is very fast
		if _, allowed := allowed[uint16(event.Kind)]; allowed {
			return false, ""
		}
		return true, "event kind not allowed"
	}
}

func PreventTimestampsInThePast(thresholdSeconds nostr.Timestamp) func(context.Context, *nostr.Event) (bool, string) {
	return func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		if nostr.Now()-event.CreatedAt > thresholdSeconds {
			return true, "event too old"
		}
		return false, ""
	}
}

func PreventTimestampsInTheFuture(thresholdSeconds nostr.Timestamp) func(context.Context, *nostr.Event) (bool, string) {
	return func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		if event.CreatedAt-nostr.Now() > thresholdSeconds {
			return true, "event too much in the future"
		}
		return false, ""
	}
}
