package plugins

import (
	"context"

	"github.com/nbd-wtf/go-nostr"
	"golang.org/x/exp/slices"
)

func NoComplexFilters(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	items := len(filter.Tags) + len(filter.Kinds)

	if items > 4 && len(filter.Tags) > 2 {
		return true, "too many things to filter for"
	}

	return false, ""
}

func NoEmptyFilters(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	c := len(filter.Kinds) + len(filter.IDs) + len(filter.Authors)
	for _, tagItems := range filter.Tags {
		c += len(tagItems)
	}
	if c == 0 {
		return true, "can't handle empty filters"
	}
	return false, ""
}

func NoSearchQueries(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	if filter.Search != "" {
		return true, "search is not supported"
	}
	return false, ""
}

func RemoveSearchQueries(ctx context.Context, filter *nostr.Filter) {
	filter.Search = ""
}

func RemoveKinds(kinds ...int) func(context.Context, *nostr.Filter) {
	return func(ctx context.Context, filter *nostr.Filter) {
		if n := len(filter.Kinds); n > 0 {
			newKinds := make([]int, 0, n)
			for i := 0; i < n; i++ {
				if k := filter.Kinds[i]; !slices.Contains(kinds, k) {
					newKinds = append(newKinds, k)
				}
			}
			filter.Kinds = newKinds
		}
	}
}

func RemoveTags(tagNames ...string) func(context.Context, *nostr.Filter) {
	return func(ctx context.Context, filter *nostr.Filter) {
		for tagName := range filter.Tags {
			if slices.Contains(tagNames, tagName) {
				delete(filter.Tags, tagName)
			}
		}
	}
}
