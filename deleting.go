package khatru

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

func (rl *Relay) handleDeleteRequest(ctx context.Context, evt *nostr.Event) error {
	// event deletion -- nip09
	for _, tag := range evt.Tags {
		if len(tag) >= 2 {
			var f nostr.Filter

			switch tag[0] {
			case "e":
				f = nostr.Filter{IDs: []string{tag[1]}}
			case "a":
				spl := strings.Split(tag[1], ":")
				if len(spl) != 3 {
					continue
				}
				kind, err := strconv.Atoi(spl[0])
				if err != nil {
					continue
				}
				author := spl[1]
				identifier := spl[2]
				f = nostr.Filter{
					Kinds:   []int{kind},
					Authors: []string{author},
					Tags:    nostr.TagMap{"d": []string{identifier}},
					Until:   &evt.CreatedAt,
				}
			default:
				continue
			}

			ctx := context.WithValue(ctx, internalCallKey, struct{}{})
			for _, query := range rl.QueryEvents {
				ch, err := query(ctx, f)
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
				if !acceptDeletion {
					msg = "you are not the author of this event"
				}
				// but if we have a function to overwrite this outcome, use that instead
				for _, odo := range rl.OverwriteDeletionOutcome {
					acceptDeletion, msg = odo(ctx, target, evt)
				}

				if acceptDeletion {
					// delete it
					for _, del := range rl.DeleteEvent {
						if err := del(ctx, target); err != nil {
							return err
						}
					}

					// if it was tracked to be expired that is not needed anymore
					rl.expirationManager.removeEvent(target.ID)
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
