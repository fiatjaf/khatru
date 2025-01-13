package khatru

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip40"
)

type expiringEvent struct {
	id        string
	expiresAt nostr.Timestamp
}

type expiringEventHeap []expiringEvent

func (h expiringEventHeap) Len() int           { return len(h) }
func (h expiringEventHeap) Less(i, j int) bool { return h[i].expiresAt < h[j].expiresAt }
func (h expiringEventHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *expiringEventHeap) Push(x interface{}) {
	*h = append(*h, x.(expiringEvent))
}

func (h *expiringEventHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type ExpirationManager struct {
	events          expiringEventHeap
	mu              sync.Mutex
	relay           *Relay
	initialScanDone bool
}

func newExpirationManager(relay *Relay) *ExpirationManager {
	return &ExpirationManager{
		events: make(expiringEventHeap, 0),
		relay:  relay,
	}
}

func (em *ExpirationManager) start(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !em.initialScanDone {
				em.initialScan(ctx)
				em.initialScanDone = true
			}

			em.checkExpiredEvents(ctx)
		}
	}
}

func (em *ExpirationManager) initialScan(ctx context.Context) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// query all events
	for _, query := range em.relay.QueryEvents {
		ch, err := query(ctx, nostr.Filter{})
		if err != nil {
			continue
		}

		for evt := range ch {
			if expiresAt := nip40.GetExpiration(evt.Tags); expiresAt != -1 {
				heap.Push(&em.events, expiringEvent{
					id:        evt.ID,
					expiresAt: expiresAt,
				})
			}
		}
	}

	heap.Init(&em.events)
}

func (em *ExpirationManager) checkExpiredEvents(ctx context.Context) {
	em.mu.Lock()
	defer em.mu.Unlock()

	now := nostr.Now()

	// keep deleting events from the heap as long as they're expired
	for em.events.Len() > 0 {
		next := em.events[0]
		if now < next.expiresAt {
			break
		}

		heap.Pop(&em.events)

		for _, query := range em.relay.QueryEvents {
			ch, err := query(ctx, nostr.Filter{IDs: []string{next.id}})
			if err != nil {
				continue
			}

			if evt := <-ch; evt != nil {
				for _, del := range em.relay.DeleteEvent {
					del(ctx, evt)
				}
			}
			break
		}
	}
}

func (em *ExpirationManager) trackEvent(evt *nostr.Event) {
	if expiresAt := nip40.GetExpiration(evt.Tags); expiresAt != -1 {
		em.mu.Lock()
		heap.Push(&em.events, expiringEvent{
			id:        evt.ID,
			expiresAt: expiresAt,
		})
		em.mu.Unlock()
	}
}
