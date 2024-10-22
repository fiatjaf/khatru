package policies

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// PreventTooManyIndexableTags returns a function that can be used as a RejectFilter that will reject
// events with more indexable (single-character) tags than the specified number.
//
// If ignoreKinds is given this restriction will not apply to these kinds (useful for allowing a bigger).
// If onlyKinds is given then all other kinds will be ignored.
func PreventTooManyIndexableTags(max int, ignoreKinds []int, onlyKinds []int) func(context.Context, *nostr.Event) (bool, string) {
	slices.Sort(ignoreKinds)
	slices.Sort(onlyKinds)

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
func RestrictToSpecifiedKinds(allowEphemeral bool, kinds ...uint16) func(context.Context, *nostr.Event) (bool, string) {
	// sort the kinds in increasing order
	slices.Sort(kinds)

	return func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		if allowEphemeral && event.IsEphemeral() {
			return false, ""
		}

		if _, allowed := slices.BinarySearch(kinds, uint16(event.Kind)); allowed {
			return false, ""
		}

		return true, fmt.Sprintf("received event kind %d not allowed", event.Kind)
	}
}

func PreventTimestampsInThePast(threshold time.Duration) func(context.Context, *nostr.Event) (bool, string) {
	thresholdSeconds := nostr.Timestamp(threshold.Seconds())
	return func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		if nostr.Now()-event.CreatedAt > thresholdSeconds {
			return true, "event too old"
		}
		return false, ""
	}
}

func PreventTimestampsInTheFuture(threshold time.Duration) func(context.Context, *nostr.Event) (bool, string) {
	thresholdSeconds := nostr.Timestamp(threshold.Seconds())
	return func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		if event.CreatedAt-nostr.Now() > thresholdSeconds {
			return true, "event too much in the future"
		}
		return false, ""
	}
}

func RejectEventsWithBase64Media(ctx context.Context, evt *nostr.Event) (bool, string) {
	return strings.Contains(evt.Content, "data:image/") || strings.Contains(evt.Content, "data:video/"), "event with base64 media"
}
