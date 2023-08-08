package plugins

import (
	"context"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

func NoPrefixFilters(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	for _, id := range filter.IDs {
		if len(id) != 64 {
			return true, fmt.Sprintf("filters can only contain full ids")
		}
	}
	for _, pk := range filter.Authors {
		if len(pk) != 64 {
			return true, fmt.Sprintf("filters can only contain full pubkeys")
		}
	}

	return false, ""
}

func NoComplexFilters(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	items := len(filter.Tags) + len(filter.Kinds)

	if items > 4 && len(filter.Tags) > 2 {
		return true, "too many things to filter for"
	}

	return false, ""
}
