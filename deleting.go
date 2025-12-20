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
	ctx = context.WithValue(ctx, internalCallKey, struct{}{})
	targets := make(map[string]*nostr.Event)
	var rejectMsg string

	for _, tag := range evt.Tags {
		if len(tag) < 2 {
			continue
		}

		var f nostr.Filter
		switch tag[0] {
		case "e":
			if tag[1] == "" {
				continue
			}
			f = nostr.Filter{IDs: []string{tag[1]}}
		case "a":
			spl := strings.SplitN(tag[1], ":", 3)
			if len(spl) != 3 {
				continue
			}
			kind, err := strconv.Atoi(spl[0])
			if err != nil {
				continue
			}
			author := spl[1]
			identifier := spl[2]
			if author == "" || identifier == "" {
				continue
			}
			f = nostr.Filter{
				Kinds:   []int{kind},
				Authors: []string{author},
				Tags:    nostr.TagMap{"d": []string{identifier}},
				Until:   &evt.CreatedAt,
			}
		default:
			continue
		}

		for _, query := range rl.QueryEvents {
			ch, err := query(ctx, f)
			if err != nil || ch == nil {
				continue
			}

			for target := range ch {
				if target == nil {
					continue
				}

				// got the event, now check if the user can delete it
				acceptDeletion := target.PubKey == evt.PubKey
				msg := ""
				if !acceptDeletion {
					msg = "you are not the author of this event"
				}
				// but if we have a function to overwrite this outcome, use that instead
				for _, odo := range rl.OverwriteDeletionOutcome {
					acceptDeletion, msg = odo(ctx, target, evt)
				}

				if !acceptDeletion {
					if msg == "" {
						msg = "you are not the author of this event"
					}
					if rejectMsg == "" {
						rejectMsg = msg
					}
					continue
				}

				// keep the first event per id
				if _, ok := targets[target.ID]; !ok {
					targets[target.ID] = target
				}
			}
		}
	}

	if rejectMsg != "" {
		return fmt.Errorf("blocked: %s", rejectMsg)
	}

	for _, target := range targets {
		// delete it
		for _, del := range rl.DeleteEvent {
			if err := del(ctx, target); err != nil {
				return err
			}
		}

		// if it was tracked to be expired that is not needed anymore
		rl.expirationManager.removeEvent(target.ID)
	}

	return nil
}
