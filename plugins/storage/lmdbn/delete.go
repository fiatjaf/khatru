package lmdbn

import (
	"context"
	"encoding/hex"

	"github.com/bmatsuo/lmdb-go/lmdb"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nson"
)

func (b *LMDBBackend) DeleteEvent(ctx context.Context, evt *nostr.Event) error {
	err := b.lmdbEnv.Update(func(txn *lmdb.Txn) error {
		id, _ := hex.DecodeString(evt.ID)
		idx, err := txn.Get(b.indexId, id)
		if operr, ok := err.(*lmdb.OpError); ok && operr.Errno == lmdb.NotFound {
			// we already do not have this
			return nil
		}
		if err != nil {
			return err
		}

		// fetch the event
		val, err := txn.Get(b.rawEventStore, idx)
		if err != nil {
			return err
		}

		evt := &nostr.Event{}
		if err := nson.Unmarshal(string(val), evt); err != nil {
			return err
		}

		// calculate all index keys we have for this event and delete them
		for _, k := range b.getIndexKeysForEvent(evt) {
			if err := txn.Del(k.dbi, k.key, nil); err != nil {
				return err
			}
		}

		// delete the raw event
		return txn.Del(b.rawEventStore, idx, nil)
	})
	if err != nil {
		return err
	}

	return nil
}
