package khatru

import (
	"context"
	"errors"

	"github.com/nbd-wtf/go-nostr"
)

func (rl *Relay) handleEphemeral(ctx context.Context, evt *nostr.Event) error {
	for _, reject := range rl.RejectEvent {
		if reject, msg := reject(ctx, evt); reject {
			if msg == "" {
				return errors.New("blocked: no reason")
			} else {
				return errors.New(nostr.NormalizeOKMessage(msg, "blocked"))
			}
		}
	}

	for _, oee := range rl.OnEphemeralEvent {
		oee(ctx, evt)
	}

	return nil
}
