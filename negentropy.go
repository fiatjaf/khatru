package khatru

import (
	"context"
	"errors"
	"fmt"

	"github.com/fiatjaf/eventstore"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip77/negentropy"
	"github.com/nbd-wtf/go-nostr/nip77/negentropy/storage/vector"
)

type NegentropySession struct {
	neg           *negentropy.Negentropy
	postponeClose func()
}

func (rl *Relay) startNegentropySession(ctx context.Context, filter nostr.Filter) (*vector.Vector, error) {
	ctx = eventstore.SetNegentropy(ctx)

	// do the same overwrite/reject flow we do in normal REQs
	for _, ovw := range rl.OverwriteFilter {
		ovw(ctx, &filter)
	}
	if filter.LimitZero {
		return nil, fmt.Errorf("invalid limit 0")
	}
	for _, reject := range rl.RejectFilter {
		if reject, msg := reject(ctx, filter); reject {
			return nil, errors.New(nostr.NormalizeOKMessage(msg, "blocked"))
		}
	}

	// fetch events and add them to a negentropy Vector store
	vec := vector.New()
	for _, query := range rl.QueryEvents {
		ch, err := query(ctx, filter)
		if err != nil {
			continue
		} else if ch == nil {
			continue
		}

		for event := range ch {
			// since the goal here is to sync databases we won't do fancy stuff like overwrite events
			vec.Insert(event.CreatedAt, event.ID)
		}
	}
	vec.Seal()

	return vec, nil
}
