package lmdbn

import (
	"context"
	"fmt"

	"github.com/bmatsuo/lmdb-go/lmdb"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nson"
)

func (b *LMDBBackend) SaveEvent(ctx context.Context, evt *nostr.Event) error {
	// sanity checking
	if evt.CreatedAt > maxuint32 || evt.Kind > maxuint16 {
		return fmt.Errorf("event with values out of expected boundaries")
	}

	return b.lmdbEnv.Update(func(txn *lmdb.Txn) error {
		nson, err := nson.Marshal(evt)
		if err != nil {
			return err
		}

		idx := b.Serial()
		// raw event store
		if err := txn.Put(b.rawEventStore, idx, []byte(nson), 0); err != nil {
			return err
		}

		for _, k := range b.getIndexKeysForEvent(evt) {
			if err := txn.Put(k.dbi, k.key, idx, 0); err != nil {
				return err
			}
		}

		return nil
	})
}
