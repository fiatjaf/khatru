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

	if nostr.IsEphemeralKind(evt.Kind) {
		return false, rl.handleEphemeral(ctx, evt)
	} else {
		return rl.handleNormal(ctx, evt)
	}
}

func (rl *Relay) handleNormal(ctx context.Context, evt *nostr.Event) (skipBroadcast bool, writeError error) {
	for _, reject := range rl.RejectEvent {
		if reject, msg := reject(ctx, evt); reject {
			if msg == "" {
				return true, errors.New("blocked: no reason")
			} else {
				return true, errors.New(nostr.NormalizeOKMessage(msg, "blocked"))
			}
		}
	}

	// Check to see if the event has been deleted by ID
	for _, query := range rl.QueryEvents {
		ch, err := query(ctx, nostr.Filter{
			Kinds: []int{5},
			Tags:  nostr.TagMap{"#e": []string{evt.ID}},
		})
		if err != nil {
			continue
		}
		target := <-ch
		if target == nil {
			continue
		}

		return true, errors.New("blocked: this event has been deleted")
	}

	// will store
	// regular kinds are just saved directly
	if nostr.IsRegularKind(evt.Kind) {
		for _, store := range rl.StoreEvent {
			if err := store(ctx, evt); err != nil {
				switch err {
				case eventstore.ErrDupEvent:
					return true, nil
				default:
					return false, fmt.Errorf("%s", nostr.NormalizeOKMessage(err.Error(), "error"))
				}
			}
		}
	} else {
		// Check to see if the event has been deleted by address
		for _, query := range rl.QueryEvents {
			dTagValue := ""
			for _, tag := range evt.Tags {
				if len(tag) > 0 && tag[0] == "d" {
					dTagValue = tag[1]
					break
				}
			}

			address := fmt.Sprintf("%d:%s:%s", evt.Kind, evt.PubKey, dTagValue)
			ch, err := query(ctx, nostr.Filter{
				Kinds: []int{5},
				Since: &evt.CreatedAt,
				Tags:  nostr.TagMap{"#a": []string{address}},
			})
			if err != nil {
				continue
			}
			target := <-ch
			if target == nil {
				continue
			}

			return true, errors.New("blocked: this event has been deleted")
		}

		// otherwise it's a replaceable -- so we'll use the replacer functions if we have any
		if len(rl.ReplaceEvent) > 0 {
			for _, repl := range rl.ReplaceEvent {
				if err := repl(ctx, evt); err != nil {
					switch err {
					case eventstore.ErrDupEvent:
						return true, nil
					default:
						return false, fmt.Errorf("%s", nostr.NormalizeOKMessage(err.Error(), "error"))
					}
				}
			}
		} else {
			// otherwise do it the manual way
			filter := nostr.Filter{Limit: 1, Kinds: []int{evt.Kind}, Authors: []string{evt.PubKey}}
			if nostr.IsAddressableKind(evt.Kind) {
				// when addressable, add the "d" tag to the filter
				filter.Tags = nostr.TagMap{"d": []string{evt.Tags.GetD()}}
			}

			// now we fetch old events and delete them
			shouldStore := true
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
					} else {
						// we found a more recent event, so we won't delete it and also will not store this new one
						shouldStore = false
					}
				}
			}

			// store
			if shouldStore {
				for _, store := range rl.StoreEvent {
					if saveErr := store(ctx, evt); saveErr != nil {
						switch saveErr {
						case eventstore.ErrDupEvent:
							return true, nil
						default:
							return false, fmt.Errorf("%s", nostr.NormalizeOKMessage(saveErr.Error(), "error"))
						}
					}
				}
			}
		}
	}

	for _, ons := range rl.OnEventSaved {
		ons(ctx, evt)
	}

	// track event expiration if applicable
	rl.expirationManager.trackEvent(evt)

	return false, nil
}
