package badgern

import (
	"context"

	"github.com/dgraph-io/badger/v4"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nson"
)

func (b *BadgerBackend) SaveEvent(ctx context.Context, evt *nostr.Event) error {
	return b.Update(func(txn *badger.Txn) error {
		nson, err := nson.Marshal(evt)
		if err != nil {
			return err
		}

		idx := b.Serial()
		// raw event store
		if err := txn.Set(idx, []byte(nson)); err != nil {
			return err
		}

		for _, k := range getIndexKeysForEvent(evt, idx[1:]) {
			if err := txn.Set(k, nil); err != nil {
				return err
			}
		}

		return nil
	})
}
