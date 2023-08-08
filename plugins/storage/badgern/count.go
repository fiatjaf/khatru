package badgern

import (
	"context"
	"encoding/binary"

	"github.com/dgraph-io/badger/v4"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nson"
)

func (b BadgerBackend) CountEvents(ctx context.Context, filter nostr.Filter) (int64, error) {
	var count int64 = 0

	queries, extraFilter, since, prefixLen, idxOffset, err := prepareQueries(filter)
	if err != nil {
		return 0, err
	}

	err = b.View(func(txn *badger.Txn) error {
		// iterate only through keys and in reverse order
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Reverse = true

		// actually iterate
		for _, q := range queries {
			it := txn.NewIterator(opts)
			defer it.Close()

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
						return err
					}

					panic(err)
				}

				if extraFilter == nil {
					count++
				} else {
					err = item.Value(func(val []byte) error {
						evt := &nostr.Event{}
						if err := nson.Unmarshal(string(val), evt); err != nil {
							return err
						}

						// check if this matches the other filters that were not part of the index
						if extraFilter == nil || extraFilter.Matches(evt) {
							count++
						}

						return nil
					})
					if err != nil {
						panic(err)
					}
				}
			}
		}

		return nil
	})

	return count, err
}
