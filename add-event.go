package khatru

import (
	"context"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

func (rl *Relay) AddEvent(ctx context.Context, evt *nostr.Event) error {
	if evt == nil {
		return fmt.Errorf("event is nil")
	}

	msg := ""
	rejecting := false
	for _, reject := range rl.RejectEvent {
		rejecting, msg = reject(ctx, evt)
		if rejecting {
			break
		}
	}

	if rejecting {
		if msg == "" {
			msg = "no reason"
		}
		return fmt.Errorf(msg)
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
				previous := <-ch
				if previous != nil {
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
					previous := <-ch
					if previous != nil {
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
				case ErrDupEvent:
					return nil
				default:
					errmsg := saveErr.Error()
					if nip20prefixmatcher.MatchString(errmsg) {
						return saveErr
					} else {
						return fmt.Errorf("error: failed to save (%s)", errmsg)
					}
				}
			}
		}

		for _, ons := range rl.OnEventSaved {
			ons(ctx, evt)
		}
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
