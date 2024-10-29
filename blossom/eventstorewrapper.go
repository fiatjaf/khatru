package blossom

import (
	"context"
	"strconv"

	"github.com/fiatjaf/eventstore"
	"github.com/nbd-wtf/go-nostr"
)

// EventStoreBlobIndexWrapper uses fake events to keep track of what blobs we have stored and who owns them
type EventStoreBlobIndexWrapper struct {
	eventstore.Store

	ServiceURL string
}

func (es EventStoreBlobIndexWrapper) Keep(ctx context.Context, blob BlobDescriptor, pubkey string) error {
	ch, err := es.Store.QueryEvents(ctx, nostr.Filter{Authors: []string{pubkey}, Kinds: []int{24242}, Tags: nostr.TagMap{"x": []string{blob.SHA256}}})
	if err != nil {
		return err
	}

	if <-ch == nil {
		// doesn't exist, save
		evt := &nostr.Event{
			PubKey: pubkey,
			Kind:   24242,
			Tags: nostr.Tags{
				{"x", blob.SHA256},
				{"type", blob.Type},
				{"size", strconv.Itoa(blob.Size)},
			},
			CreatedAt: blob.Uploaded,
		}
		evt.ID = evt.GetID()
		es.Store.SaveEvent(ctx, evt)
	}

	return nil
}

func (es EventStoreBlobIndexWrapper) List(ctx context.Context, pubkey string) (chan BlobDescriptor, error) {
	ech, err := es.Store.QueryEvents(ctx, nostr.Filter{Authors: []string{pubkey}, Kinds: []int{24242}})
	if err != nil {
		return nil, err
	}

	ch := make(chan BlobDescriptor)

	go func() {
		for evt := range ech {
			ch <- es.parseEvent(evt)
		}
		close(ch)
	}()

	return ch, nil
}

func (es EventStoreBlobIndexWrapper) Get(ctx context.Context, sha256 string) (*BlobDescriptor, error) {
	ech, err := es.Store.QueryEvents(ctx, nostr.Filter{Tags: nostr.TagMap{"x": []string{sha256}}, Kinds: []int{24242}, Limit: 1})
	if err != nil {
		return nil, err
	}

	evt := <-ech
	if evt != nil {
		bd := es.parseEvent(evt)
		return &bd, nil
	}

	return nil, nil
}

func (es EventStoreBlobIndexWrapper) Delete(ctx context.Context, sha256 string, pubkey string) error {
	ech, err := es.Store.QueryEvents(ctx, nostr.Filter{Authors: []string{pubkey}, Tags: nostr.TagMap{"x": []string{sha256}}, Kinds: []int{24242}, Limit: 1})
	if err != nil {
		return err
	}

	evt := <-ech
	if evt != nil {
		return es.Store.DeleteEvent(ctx, evt)
	}

	return nil
}

func (es EventStoreBlobIndexWrapper) parseEvent(evt *nostr.Event) BlobDescriptor {
	hhash := evt.Tags[0][1]
	mimetype := evt.Tags[1][1]
	ext := getExtension(mimetype)
	size, _ := strconv.Atoi(evt.Tags[2][1])

	return BlobDescriptor{
		Owner:    evt.PubKey,
		Uploaded: evt.CreatedAt,
		URL:      es.ServiceURL + "/" + hhash + ext,
		SHA256:   hhash,
		Type:     mimetype,
		Size:     size,
	}
}
