package lmdbn

import (
	"encoding/binary"
	"encoding/hex"
	"sync/atomic"

	"github.com/bmatsuo/lmdb-go/lmdb"
	"github.com/nbd-wtf/go-nostr"
)

const (
	maxuint16 = 65535
	maxuint32 = 4294967295
)

type LMDBBackend struct {
	Path     string
	MaxLimit int

	lmdbEnv *lmdb.Env

	rawEventStore   lmdb.DBI
	indexCreatedAt  lmdb.DBI
	indexId         lmdb.DBI
	indexKind       lmdb.DBI
	indexPubkey     lmdb.DBI
	indexPubkeyKind lmdb.DBI
	indexTag        lmdb.DBI

	lastId atomic.Uint32
}

func (b *LMDBBackend) Init() error {
	if b.MaxLimit == 0 {
		b.MaxLimit = 500
	}

	// open lmdb
	env, err := lmdb.NewEnv()
	if err != nil {
		return err
	}

	env.SetMaxDBs(7)
	env.SetMaxReaders(500)
	env.SetMapSize(1 << 38) // ~273GB

	err = env.Open(b.Path, lmdb.NoTLS, 0644)
	if err != nil {
		return err
	}
	b.lmdbEnv = env

	// open each db
	if err := b.lmdbEnv.Update(func(txn *lmdb.Txn) error {
		if dbi, err := txn.OpenDBI("raw", lmdb.Create); err != nil {
			return err
		} else {
			b.rawEventStore = dbi
			return nil
		}
	}); err != nil {
		return err
	}
	if err := b.lmdbEnv.Update(func(txn *lmdb.Txn) error {
		if dbi, err := txn.OpenDBI("created_at", lmdb.Create); err != nil {
			return err
		} else {
			b.indexCreatedAt = dbi
		}
		if dbi, err := txn.OpenDBI("id", lmdb.Create); err != nil {
			return err
		} else {
			b.indexId = dbi
		}
		if dbi, err := txn.OpenDBI("kind", lmdb.Create); err != nil {
			return err
		} else {
			b.indexKind = dbi
		}
		if dbi, err := txn.OpenDBI("pubkey", lmdb.Create); err != nil {
			return err
		} else {
			b.indexPubkey = dbi
		}
		if dbi, err := txn.OpenDBI("pubkeyKind", lmdb.Create); err != nil {
			return err
		} else {
			b.indexPubkeyKind = dbi
		}
		if dbi, err := txn.OpenDBI("tag", lmdb.Create); err != nil {
			return err
		} else {
			b.indexTag = dbi
		}
		return nil
	}); err != nil {
		return err
	}

	// get lastId
	if err := b.lmdbEnv.View(func(txn *lmdb.Txn) error {
		txn.RawRead = true
		cursor, err := txn.OpenCursor(b.rawEventStore)
		if err != nil {
			return err
		}
		defer cursor.Close()
		k, _, err := cursor.Get(nil, nil, lmdb.Last)
		if operr, ok := err.(*lmdb.OpError); ok && operr.Errno == lmdb.NotFound {
			// nothing found, so we're at zero
			return nil
		}
		if err != nil {
		}
		b.lastId.Store(binary.BigEndian.Uint32(k))

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (b *LMDBBackend) Close() {
	b.lmdbEnv.Close()
}

func (b *LMDBBackend) Serial() []byte {
	v := b.lastId.Add(1)
	vb := make([]byte, 4)
	binary.BigEndian.PutUint32(vb[:], uint32(v))
	return vb
}

type key struct {
	dbi lmdb.DBI
	key []byte
}

func (b *LMDBBackend) getIndexKeysForEvent(evt *nostr.Event) []key {
	keys := make([]key, 0, 18)

	// indexes
	{
		// ~ by id
		k, _ := hex.DecodeString(evt.ID)
		keys = append(keys, key{dbi: b.indexId, key: k})
	}

	{
		// ~ by pubkey+date
		pubkey, _ := hex.DecodeString(evt.PubKey)
		k := make([]byte, 32+4)
		copy(k[:], pubkey)
		binary.BigEndian.PutUint32(k[32:], uint32(evt.CreatedAt))
		keys = append(keys, key{dbi: b.indexPubkey, key: k})
	}

	{
		// ~ by kind+date
		k := make([]byte, 2+4)
		binary.BigEndian.PutUint16(k[:], uint16(evt.Kind))
		binary.BigEndian.PutUint32(k[2:], uint32(evt.CreatedAt))
		keys = append(keys, key{dbi: b.indexKind, key: k})
	}

	{
		// ~ by pubkey+kind+date
		pubkey, _ := hex.DecodeString(evt.PubKey)
		k := make([]byte, 32+2+4)
		copy(k[:], pubkey)
		binary.BigEndian.PutUint16(k[32:], uint16(evt.Kind))
		binary.BigEndian.PutUint32(k[32+2:], uint32(evt.CreatedAt))
		keys = append(keys, key{dbi: b.indexPubkeyKind, key: k})
	}

	// ~ by tagvalue+date
	for _, tag := range evt.Tags {
		if len(tag) < 2 || len(tag[0]) != 1 || len(tag[1]) == 0 || len(tag[1]) > 100 {
			continue
		}

		var v []byte
		if vb, _ := hex.DecodeString(tag[1]); len(vb) == 32 {
			// store value as bytes
			v = vb
		} else {
			v = []byte(tag[1])
		}

		k := make([]byte, len(v)+4)
		copy(k[:], v)
		binary.BigEndian.PutUint32(k[len(v):], uint32(evt.CreatedAt))
		keys = append(keys, key{dbi: b.indexTag, key: k})
	}

	{
		// ~ by date only
		k := make([]byte, 4)
		binary.BigEndian.PutUint32(k[:], uint32(evt.CreatedAt))
		keys = append(keys, key{dbi: b.indexCreatedAt, key: k})
	}

	return keys
}
