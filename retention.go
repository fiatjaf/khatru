package khatru

import (
	"context"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

type retentionManager struct {
	relay    *Relay
	interval time.Duration
}

func newRetentionManager(relay *Relay) *retentionManager {
	return &retentionManager{
		relay:    relay,
		interval: time.Hour * 24,
	}
}

func (rm *retentionManager) start(ctx context.Context) {
	ticker := time.NewTicker(rm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(rm.relay.Info.Retention) == 0 {
				continue
			}

			for _, rr := range rm.relay.Info.Retention {
				for _, query := range rm.relay.QueryEvents {
					kinds := []int{}

					for _, k := range rr.Kinds {
						if len(kinds) == 1 {
							kinds = append(kinds, k[0])
						}

						if len(k) == 2 {
							for {
								start := k[0]
								end := k[1]

								if start < end {
									for i := start; i <= end; i++ {
										kinds = append(kinds, i)
									}
								}
							}
						}
					}
					ch, err := query(ctx, nostr.Filter{
						Limit: rr.Count,
						Kinds: kinds,
					})
					if err != nil {
						continue
					}

					if evt := <-ch; evt != nil {
						if time.Since(evt.CreatedAt.Time()) >= time.Duration(rr.Time) {
							for _, del := range rm.relay.DeleteEvent {
								del(ctx, evt)
							}
						}
					}
					break
				}
			}
		}
	}
}
