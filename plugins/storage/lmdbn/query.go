package lmdbn

import (
	"bytes"
	"container/heap"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/bmatsuo/lmdb-go/lmdb"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nson"
)

type query struct {
	i             int
	prefix        []byte
	startingPoint []byte
	results       chan *nostr.Event
	skipTimestamp bool
}

type queryEvent struct {
	*nostr.Event
	query int
}

func (b *LMDBBackend) QueryEvents(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
	ch := make(chan *nostr.Event)

	dbi, queries, extraFilter, since, prefixLen, err := b.prepareQueries(filter)
	if err != nil {
		return nil, err
	}

	go func() {
		err := b.lmdbEnv.View(func(txn *lmdb.Txn) error {
			// actually iterate
			cursorClosers := make([]func(), len(queries))
			for i, q := range queries {
				go func(i int, q query) {
					defer close(q.results)

					cursor, err := txn.OpenCursor(dbi)
					if err != nil {
						return
					}
					cursorClosers[i] = cursor.Close

					var k []byte
					var idx []byte
					var iterr error

					if _, _, errsr := cursor.Get(q.startingPoint, nil, lmdb.SetRange); errsr != nil {
						if operr, ok := errsr.(*lmdb.OpError); !ok || operr.Errno != lmdb.NotFound {
							// in this case it's really an error
							panic(err)
						} else {
							// we're at the end and we just want notes before this,
							// so we just need to set the cursor the last key, this is not a real error
							k, idx, iterr = cursor.Get(nil, nil, lmdb.Last)
						}
					} else {
						// move one back as the first step
						k, idx, iterr = cursor.Get(nil, nil, lmdb.Prev)
					}

					for {
						select {
						case <-ctx.Done():
							break
						default:
						}

						// we already have a k and a v and an err from the cursor setup, so check and use these
						if iterr != nil || !bytes.Equal(q.prefix, k[0:prefixLen]) {
							return
						}

						if !q.skipTimestamp {
							createdAt := binary.BigEndian.Uint32(k[prefixLen:])
							if createdAt < since {
								break
							}
						}

						// fetch actual event
						val, err := txn.Get(b.rawEventStore, idx)
						if err != nil {
							panic(err)
						}

						evt := &nostr.Event{}
						if err := nson.Unmarshal(string(val), evt); err != nil {
							panic(err)
						}

						// check if this matches the other filters that were not part of the index
						if extraFilter == nil || extraFilter.Matches(evt) {
							q.results <- evt
						}

						// move one back (we'll look into k and v and err in the next iteration)
						k, idx, iterr = cursor.Get(nil, nil, lmdb.Prev)
					}
				}(i, q)
			}

			// max number of events we'll return
			limit := b.MaxLimit
			if filter.Limit > 0 && filter.Limit < limit {
				limit = filter.Limit
			}

			// receive results and ensure we only return the most recent ones always
			emittedEvents := 0

			// first pass
			emitQueue := make(priorityQueue, 0, len(queries)+limit)
			for _, q := range queries {
				evt, ok := <-q.results
				if ok {
					emitQueue = append(emitQueue, &queryEvent{Event: evt, query: q.i})
				}
			}

			// now it's a good time to schedule this
			defer func() {
				close(ch)
				for _, cclose := range cursorClosers {
					cclose()
				}
			}()

			// queue may be empty here if we have literally nothing
			if len(emitQueue) == 0 {
				return nil
			}

			heap.Init(&emitQueue)

			// iterate until we've emitted all events required
			for {
				// emit latest event in queue
				latest := emitQueue[0]
				ch <- latest.Event

				// stop when reaching limit
				emittedEvents++
				if emittedEvents >= limit {
					break
				}

				// fetch a new one from query results and replace the previous one with it
				if evt, ok := <-queries[latest.query].results; ok {
					emitQueue[0].Event = evt
					heap.Fix(&emitQueue, 0)
				} else {
					// if this query has no more events we just remove this and proceed normally
					heap.Remove(&emitQueue, 0)

					// check if the list is empty and end
					if len(emitQueue) == 0 {
						break
					}
				}
			}

			return nil
		})
		if err != nil {
			panic(err)
		}
	}()

	return ch, nil
}

type priorityQueue []*queryEvent

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].CreatedAt > pq[j].CreatedAt
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *priorityQueue) Push(x any) {
	item := x.(*queryEvent)
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*pq = old[0 : n-1]
	return item
}

func (b *LMDBBackend) prepareQueries(filter nostr.Filter) (
	dbi lmdb.DBI,
	queries []query,
	extraFilter *nostr.Filter,
	since uint32,
	prefixLen int,
	err error,
) {
	if len(filter.IDs) > 0 {
		dbi = b.indexId
		queries = make([]query, len(filter.IDs))
		for i, idHex := range filter.IDs {
			prefix, _ := hex.DecodeString(idHex)
			if len(prefix) != 32 {
				return dbi, nil, nil, 0, 0, fmt.Errorf("invalid id '%s'", idHex)
			}
			queries[i] = query{i: i, prefix: prefix, skipTimestamp: true}
		}
	} else if len(filter.Authors) > 0 {
		if len(filter.Kinds) == 0 {
			dbi = b.indexPubkey
			queries = make([]query, len(filter.Authors))
			for i, pubkeyHex := range filter.Authors {
				prefix, _ := hex.DecodeString(pubkeyHex)
				if len(prefix) != 32 {
					return dbi, nil, nil, 0, 0, fmt.Errorf("invalid pubkey '%s'", pubkeyHex)
				}
				queries[i] = query{i: i, prefix: prefix}
			}
		} else {
			dbi = b.indexPubkeyKind
			queries = make([]query, len(filter.Authors)*len(filter.Kinds))
			i := 0
			for _, pubkeyHex := range filter.Authors {
				for _, kind := range filter.Kinds {
					pubkey, _ := hex.DecodeString(pubkeyHex)
					if len(pubkey) != 32 {
						return dbi, nil, nil, 0, 0, fmt.Errorf("invalid pubkey '%s'", pubkeyHex)
					}
					prefix := make([]byte, 32+2)
					copy(prefix[:], pubkey)
					binary.BigEndian.PutUint16(prefix[+32:], uint16(kind))
					queries[i] = query{i: i, prefix: prefix}
					i++
				}
			}
		}
		extraFilter = &nostr.Filter{Tags: filter.Tags}
	} else if len(filter.Tags) > 0 {
		dbi = b.indexTag
		queries = make([]query, len(filter.Tags))
		extraFilter = &nostr.Filter{Kinds: filter.Kinds}
		i := 0
		for _, values := range filter.Tags {
			for _, value := range values {
				bv, _ := hex.DecodeString(value)
				var size int
				if len(bv) == 32 {
					// hex tag
					size = 32
				} else {
					// string tag
					bv = []byte(value)
					size = len(bv)
				}
				prefix := make([]byte, size)
				copy(prefix[:], bv)
				queries[i] = query{i: i, prefix: prefix}
				i++
			}
		}
	} else if len(filter.Kinds) > 0 {
		dbi = b.indexKind
		queries = make([]query, len(filter.Kinds))
		for i, kind := range filter.Kinds {
			prefix := make([]byte, 2)
			binary.BigEndian.PutUint16(prefix[:], uint16(kind))
			queries[i] = query{i: i, prefix: prefix}
		}
	} else {
		dbi = b.indexCreatedAt
		queries = make([]query, 1)
		prefix := make([]byte, 0)
		queries[0] = query{i: 0, prefix: prefix}
		extraFilter = nil
	}

	prefixLen = len(queries[0].prefix)

	var until uint32 = 4294967295
	if filter.Until != nil {
		if fu := uint32(*filter.Until); fu < until {
			until = fu + 1
		}
	}
	for i, q := range queries {
		queries[i].startingPoint = binary.BigEndian.AppendUint32(q.prefix, uint32(until))
		queries[i].results = make(chan *nostr.Event, 12)
	}

	// this is where we'll end the iteration
	if filter.Since != nil {
		if fs := uint32(*filter.Since); fs > since {
			since = fs
		}
	}

	return dbi, queries, extraFilter, since, prefixLen, nil
}
