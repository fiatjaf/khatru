package plugins

import (
	"context"

	"github.com/nbd-wtf/go-nostr"
)

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
