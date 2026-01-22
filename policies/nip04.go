package policies

import (
	"context"
	"slices"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
)

// RejectKind04Snoopers prevents reading NIP-04 messages from people not involved in the conversation.
func RejectKind04Snoopers(ctx context.Context, filter nostr.Filter) (bool, string) {
	// prevent kind-4 events from being returned to unauthed users,
	//   only when authentication is a thing
	if !slices.Contains(filter.Kinds, 4) {
		return false, ""
	}

	senders := filter.Authors
	receivers, _ := filter.Tags["p"]
	switch {
	case khatru.GetAuthed(ctx) == "":
		// not authenticated
		return true, "restricted: this relay does not serve kind-4 to unauthenticated users, does your client implement NIP-42?"
	case len(senders) == 1 && len(receivers) < 2 && khatru.IsAuthed(ctx, senders[0]):
		// allowed filter: authed user is sole sender (filter specifies one or all receivers)
		return false, ""
	case len(receivers) == 1 && len(senders) < 2 && khatru.IsAuthed(ctx, receivers[0]):
		// allowed filter: authed user is sole receiver (filter specifies one or all senders)
		return false, ""
	default:
		// restricted filter: do not return any events,
		//   even if other elements in filters array were not restricted).
		//   client should know better.
		return true, "restricted: authenticated user does not have authorization for requested filters."
	}
}
