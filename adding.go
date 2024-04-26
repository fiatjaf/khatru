package khatru

import (
	"context"
	"errors"
	"fmt"

	"github.com/fiatjaf/eventstore"
	"github.com/nbd-wtf/go-nostr"
)

// AddEvent sends an event through then normal add pipeline, as if it was received from a websocket.
func (rl *Relay) AddEvent(ctx context.Context, evt *nostr.Event) (skipBroadcast bool, writeError error) {
	if evt == nil {
		return false, errors.New("error: event is nil")
	}

	for _, reject := range rl.RejectEvent {
		if reject, msg := reject(ctx, evt); reject {
			if msg == "" {
				return false, errors.New("blocked: no reason")
			} else {
				return false, errors.New(nostr.NormalizeOKMessage(msg, "blocked"))
			}
		}
	}

	if 20000 <= evt.Kind && evt.Kind < 30000 {
		// do not store ephemeral events
		for _, oee := range rl.OnEphemeralEvent {
			oee(ctx, evt)
		}
	} else {
		// will store

		// but first check if we already have it
		filter := nostr.Filter{IDs: []string{evt.ID}}
		for _, query := range rl.QueryEvents {
			ch, err := query(ctx, filter)
			if err != nil {
				continue
			}
			for range ch {
				// if we run this it means we already have this event, so we just return a success and exit
				return true, nil
			}
		}

		// if it's replaceable we first delete old versions
		if evt.Kind == 0 || evt.Kind == 3 || (10000 <= evt.Kind && evt.Kind < 20000) {
			// replaceable event, delete before storing
			filter := nostr.Filter{Authors: []string{evt.PubKey}, Kinds: []int{evt.Kind}}
			for _, query := range rl.QueryEvents {
				ch, err := query(ctx, filter)
				if err != nil {
					continue
				}
				for previous := range ch {
					if isOlder(previous, evt) {
						for _, del := range rl.DeleteEvent {
							del(ctx, previous)
						}
					}
				}
			}
		} else if 30000 <= evt.Kind && evt.Kind < 40000 {
			// parameterized replaceable event, delete before storing
			d := evt.Tags.GetFirst([]string{"d", ""})
			if d == nil {
				return false, fmt.Errorf("invalid: missing 'd' tag on parameterized replaceable event")
			}

			filter := nostr.Filter{Authors: []string{evt.PubKey}, Kinds: []int{evt.Kind}, Tags: nostr.TagMap{"d": []string{(*d)[1]}}}
			for _, query := range rl.QueryEvents {
				ch, err := query(ctx, filter)
				if err != nil {
					continue
				}
				for previous := range ch {
					if isOlder(previous, evt) {
						for _, del := range rl.DeleteEvent {
							del(ctx, previous)
						}
					}
				}
			}
		}

		// store
		for _, store := range rl.StoreEvent {
			if saveErr := store(ctx, evt); saveErr != nil {
				switch saveErr {
				case eventstore.ErrDupEvent:
					return true, nil
				default:
					return false, fmt.Errorf(nostr.NormalizeOKMessage(saveErr.Error(), "error"))
				}
			}
		}

		for _, ons := range rl.OnEventSaved {
			ons(ctx, evt)
		}
	}

	return false, nil
}
