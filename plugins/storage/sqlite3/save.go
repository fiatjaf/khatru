package sqlite3

import (
	"context"
	"encoding/json"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
)

func (b *SQLite3Backend) SaveEvent(ctx context.Context, evt *nostr.Event) error {
	// insert
	tagsj, _ := json.Marshal(evt.Tags)
	res, err := b.DB.ExecContext(ctx, `
        INSERT INTO event (id, pubkey, created_at, kind, tags, content, sig)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, evt.ID, evt.PubKey, evt.CreatedAt, evt.Kind, tagsj, evt.Content, evt.Sig)
	if err != nil {
		return err
	}

	nr, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if nr == 0 {
		return khatru.ErrDupEvent
	}

	return nil
}

func (b *SQLite3Backend) BeforeSave(ctx context.Context, evt *nostr.Event) {
	// do nothing
}

func (b *SQLite3Backend) AfterSave(evt *nostr.Event) {
	// delete all but the 100 most recent ones for each key
	b.DB.Exec(`DELETE FROM event WHERE pubkey = $1 AND kind = $2 AND created_at < (
      SELECT created_at FROM event WHERE pubkey = $1
      ORDER BY created_at DESC OFFSET 100 LIMIT 1
    )`, evt.PubKey, evt.Kind)
}
