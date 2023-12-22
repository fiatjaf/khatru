package khatru

import (
	"context"
	"errors"
	"fmt"

	"github.com/fiatjaf/eventstore"
	"github.com/nbd-wtf/go-nostr"
)

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

	for _, ovw := range rl.OverwriteResponseEvent {
		ovw(ctx, evt)
	}
	notifyListeners(evt)

	return nil
}

func (rl *Relay) handleDeleteRequest(ctx context.Context, evt *nostr.Event) error {
	// event deletion -- nip09
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			// first we fetch the event
			for _, query := range rl.QueryEvents {
				ch, err := query(ctx, nostr.Filter{IDs: []string{tag[1]}})
				if err != nil {
					continue
				}
				target := <-ch
				if target == nil {
					continue
				}
				// got the event, now check if the user can delete it
				acceptDeletion := target.PubKey == evt.PubKey
				var msg string
				if acceptDeletion == false {
					msg = "you are not the author of this event"
				}
				// but if we have a function to overwrite this outcome, use that instead
				for _, odo := range rl.OverwriteDeletionOutcome {
					acceptDeletion, msg = odo(ctx, target, evt)
				}
				if acceptDeletion {
					// delete it
					for _, del := range rl.DeleteEvent {
						del(ctx, target)
					}
				} else {
					// fail and stop here
					return fmt.Errorf("blocked: %s", msg)
				}

				// don't try to query this same event again
				break
			}
		}
	}

	return nil
}
