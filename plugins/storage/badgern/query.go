package badgern

import (
	"container/heap"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/dgraph-io/badger/v4"
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

func (b BadgerBackend) QueryEvents(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
	ch := make(chan *nostr.Event)

	queries, extraFilter, since, prefixLen, idxOffset, err := prepareQueries(filter)
	if err != nil {
		return nil, err
	}

	go func() {
		err := b.View(func(txn *badger.Txn) error {
			// iterate only through keys and in reverse order
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			opts.Reverse = true

			// actually iterate
			iteratorClosers := make([]func(), len(queries))
			for i, q := range queries {
				go func(i int, q query) {
					it := txn.NewIterator(opts)
					iteratorClosers[i] = it.Close

					defer close(q.results)

					for it.Seek(q.startingPoint); it.ValidForPrefix(q.prefix); it.Next() {
						item := it.Item()
						key := item.Key()

						if !q.skipTimestamp {
							createdAt := binary.BigEndian.Uint32(key[prefixLen:idxOffset])
							if createdAt < since {
								break
							}
						}

						idx := make([]byte, 5)
						idx[0] = rawEventStorePrefix
						copy(idx[1:], key[idxOffset:])

						// fetch actual event
						item, err := txn.Get(idx)
						if err != nil {
							if err == badger.ErrDiscardedTxn {
								return
							}

							panic(err)
						}
						err = item.Value(func(val []byte) error {
							evt := &nostr.Event{}
							if err := nson.Unmarshal(string(val), evt); err != nil {
								return err
							}

							// check if this matches the other filters that were not part of the index
							if extraFilter == nil || extraFilter.Matches(evt) {
								q.results <- evt
							}

							return nil
						})
						if err != nil {
							panic(err)
						}
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
				for _, itclose := range iteratorClosers {
					itclose()
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
				if emittedEvents == limit {
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

func prepareQueries(filter nostr.Filter) (
	queries []query,
	extraFilter *nostr.Filter,
	since uint32,
	prefixLen int,
	idxOffset int,
	err error,
) {
	var index byte

	if len(filter.IDs) > 0 {
		index = indexIdPrefix
		queries = make([]query, len(filter.IDs))
		for i, idHex := range filter.IDs {
			prefix := make([]byte, 1+32)
			prefix[0] = index
			id, _ := hex.DecodeString(idHex)
			if len(id) != 32 {
				return nil, nil, 0, 0, 0, fmt.Errorf("invalid id '%s'", idHex)
			}
			copy(prefix[1:], id)
			queries[i] = query{i: i, prefix: prefix, skipTimestamp: true}
		}
	} else if len(filter.Authors) > 0 {
		if len(filter.Kinds) == 0 {
			index = indexPubkeyPrefix
			queries = make([]query, len(filter.Authors))
			for i, pubkeyHex := range filter.Authors {
				pubkey, _ := hex.DecodeString(pubkeyHex)
				if len(pubkey) != 32 {
					continue
				}
				prefix := make([]byte, 1+32)
				prefix[0] = index
				copy(prefix[1:], pubkey)
				queries[i] = query{i: i, prefix: prefix}
			}
		} else {
			index = indexPubkeyKindPrefix
			queries = make([]query, len(filter.Authors)*len(filter.Kinds))
			i := 0
			for _, pubkeyHex := range filter.Authors {
				for _, kind := range filter.Kinds {
					pubkey, _ := hex.DecodeString(pubkeyHex)
					if len(pubkey) != 32 {
						return nil, nil, 0, 0, 0, fmt.Errorf("invalid pubkey '%s'", pubkeyHex)
					}
					prefix := make([]byte, 1+32+2)
					prefix[0] = index
					copy(prefix[1:], pubkey)
					binary.BigEndian.PutUint16(prefix[1+32:], uint16(kind))
					queries[i] = query{i: i, prefix: prefix}
					i++
				}
			}
		}
		extraFilter = &nostr.Filter{Tags: filter.Tags}
	} else if len(filter.Tags) > 0 {
		index = indexTagPrefix

		// determine the size of the queries array by inspecting all tags sizes
		size := 0
		for _, values := range filter.Tags {
			size += len(values)
		}
		queries = make([]query, size)

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
				prefix := make([]byte, 1+size)
				prefix[0] = index
				copy(prefix[1:], bv)
				queries[i] = query{i: i, prefix: prefix}
				i++
			}
		}
	} else if len(filter.Kinds) > 0 {
		index = indexKindPrefix
		queries = make([]query, len(filter.Kinds))
		for i, kind := range filter.Kinds {
			prefix := make([]byte, 1+2)
			prefix[0] = index
			binary.BigEndian.PutUint16(prefix[1:], uint16(kind))
			queries[i] = query{i: i, prefix: prefix}
		}
	} else {
		index = indexCreatedAtPrefix
		queries = make([]query, 1)
		prefix := make([]byte, 1)
		prefix[0] = index
		queries[0] = query{i: 0, prefix: prefix}
		extraFilter = nil
	}

	prefixLen = len(queries[0].prefix)

	if index == indexIdPrefix {
		idxOffset = prefixLen
	} else {
		idxOffset = prefixLen + 4
	}

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

	return queries, extraFilter, since, prefixLen, idxOffset, nil
}
