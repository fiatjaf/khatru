package policies

import (
    "fmt"
	"context"
	"slices"

	"github.com/nbd-wtf/go-nostr"
)

// PreventTooManyIndexableTags returns a function that can be used as a RejectFilter that will reject
// events with more indexable (single-character) tags than the specified number.
//
// If ignoreKinds is given this restriction will not apply to these kinds (useful for allowing a bigger).
// If onlyKinds is given then all other kinds will be ignored.
func PreventTooManyIndexableTags(max int, ignoreKinds []int, onlyKinds []int) func(context.Context, *nostr.Event) (bool, string) {
	ignore := func(kind int) bool { return false }
	if len(ignoreKinds) > 0 {
		ignore = func(kind int) bool {
			_, isIgnored := slices.BinarySearch(ignoreKinds, kind)
			return isIgnored
		}
	}
	if len(onlyKinds) > 0 {
		ignore = func(kind int) bool {
			_, isApplicable := slices.BinarySearch(onlyKinds, kind)
			return !isApplicable
		}
	}

	return func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		if ignore(event.Kind) {
			return false, ""
		}

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

// PreventLargeTags rejects events that have indexable tag values greater than maxTagValueLen.
func PreventLargeTags(maxTagValueLen int) func(context.Context, *nostr.Event) (bool, string) {
	return func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		for _, tag := range event.Tags {
			if len(tag) > 1 && len(tag[0]) == 1 {
				if len(tag[1]) > maxTagValueLen {
					return true, "event contains too large tags"
				}
			}
		}
		return false, ""
	}
}

// RestrictToSpecifiedKinds returns a function that can be used as a RejectFilter that will reject
// any events with kinds different than the specified ones.
func RestrictToSpecifiedKinds(kinds ...uint16) func(context.Context, *nostr.Event) (bool, string) {
	max := 0
	min := 0
	for _, kind := range kinds {
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
			return true, fmt.Sprintf("event kind not allowed (it should be lower than %)", max)
		}
		if event.Kind < min {
			return true, fmt.Sprintf("event kind not allowed (it should be higher than %d)", min)
		}

        // Sort the kinds in increasing order
        slices.Sort(kinds)

		// hopefully this map of uint16s is very fast
		if _, allowed := slices.BinarySearch(kinds, uint16(event.Kind)); allowed {
			return false, ""
		}

        allowedKindsStringFormatted := fmt.Sprintf("%d\n", kinds)
		return true, fmt.Sprintf("Received event kind %d not allowed, only allowed are: %s", event.Kind, allowedKindsStringFormatted)
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
