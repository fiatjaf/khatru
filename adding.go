package khatru

import (
	"context"
	"errors"
	"fmt"

	"github.com/fiatjaf/eventstore"
	"github.com/nbd-wtf/go-nostr"
)

// AddEvent sends an event through then normal add pipeline, as if it was received from a websocket.
func (rl *Relay) AddEvent(ctx context.Context, evt *nostr.Event) error {
	if evt == nil {
		return errors.New("error: event is nil")
	}

	for _, reject := range rl.RejectEvent {
		if reject, msg := reject(ctx, evt); reject {
			if msg == "" {
				return errors.New("blocked: no reason")
			} else {
				return errors.New(nostr.NormalizeOKMessage(msg, "blocked"))
			}
		}
	}

	if 20000 <= evt.Kind && evt.Kind < 30000 {
		// do not store ephemeral events
		for _, oee := range rl.OnEphemeralEvent {
			oee(ctx, evt)
		}
	} else {
		if evt.Kind == 0 || evt.Kind == 3 || (10000 <= evt.Kind && evt.Kind < 20000) {
			// replaceable event, delete before storing
			for _, query := range rl.QueryEvents {
				ch, err := query(ctx, nostr.Filter{Authors: []string{evt.PubKey}, Kinds: []int{evt.Kind}})
				if err != nil {
					continue
				}
				if previous := <-ch; previous != nil && isOlder(previous, evt) {
					for _, del := range rl.DeleteEvent {
						del(ctx, previous)
					}
				}
			}
		} else if 30000 <= evt.Kind && evt.Kind < 40000 {
			// parameterized replaceable event, delete before storing
			d := evt.Tags.GetFirst([]string{"d", ""})
			if d != nil {
				for _, query := range rl.QueryEvents {
					ch, err := query(ctx, nostr.Filter{Authors: []string{evt.PubKey}, Kinds: []int{evt.Kind}, Tags: nostr.TagMap{"d": []string{d.Value()}}})
					if err != nil {
						continue
					}
					if previous := <-ch; previous != nil && isOlder(previous, evt) {
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
					return nil
				default:
					return fmt.Errorf(nostr.NormalizeOKMessage(saveErr.Error(), "error"))
				}
			}
		}

		for _, ons := range rl.OnEventSaved {
			ons(ctx, evt)
		}
	}

	return nil
}
