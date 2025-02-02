package policies

import (
	"context"
	"slices"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
)

// NoComplexFilters disallows filters with more than 2 tags.
func NoComplexFilters(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	items := len(filter.Tags) + len(filter.Kinds)

	if items > 4 && len(filter.Tags) > 2 {
		return true, "too many things to filter for"
	}

	return false, ""
}

// MustAuth requires all subscribers to be authenticated
func MustAuth(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	if khatru.GetAuthed(ctx) == "" {
		return true, "auth-required: all requests must be authenticated"
	}
	return false, ""
}

// NoEmptyFilters disallows filters that don't have at least a tag, a kind, an author or an id.
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

// AntiSyncBots tries to prevent people from syncing kind:1s from this relay to else by always
// requiring an author parameter at least.
func AntiSyncBots(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	return (len(filter.Kinds) == 0 || slices.Contains(filter.Kinds, 1)) &&
		len(filter.Authors) == 0, "an author must be specified to get their kind:1 notes"
}

func NoSearchQueries(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
	if filter.Search != "" {
		return true, "search is not supported"
	}
	return false, ""
}

func RemoveSearchQueries(ctx context.Context, filter *nostr.Filter) {
	if filter.Search != "" {
		filter.Search = ""
		filter.LimitZero = true // signals that this query should be just skipped
	}
}

func RemoveAllButKinds(kinds ...uint16) func(context.Context, *nostr.Filter) {
	return func(ctx context.Context, filter *nostr.Filter) {
		if n := len(filter.Kinds); n > 0 {
			newKinds := make([]int, 0, n)
			for i := 0; i < n; i++ {
				if k := filter.Kinds[i]; slices.Contains(kinds, uint16(k)) {
					newKinds = append(newKinds, k)
				}
			}
			filter.Kinds = newKinds
			if len(filter.Kinds) == 0 {
				filter.LimitZero = true // signals that this query should be just skipped
			}
		}
	}
}

func RemoveAllButTags(tagNames ...string) func(context.Context, *nostr.Filter) {
	return func(ctx context.Context, filter *nostr.Filter) {
		if n := len(filter.Tags); n > 0 {
			for tagName := range filter.Tags {
				if !slices.Contains(tagNames, tagName) {
					delete(filter.Tags, tagName)
				}
			}
			if len(filter.Tags) == 0 {
				filter.LimitZero = true // signals that this query should be just skipped
			}
		}
	}
}
