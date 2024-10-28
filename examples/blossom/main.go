package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/fiatjaf/eventstore/badger"
	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/blossom"
)

func main() {
	relay := khatru.NewRelay()

	db := &badger.BadgerBackend{Path: "/tmp/khatru-badger-blossom-tmp"}
	if err := db.Init(); err != nil {
		panic(err)
	}

	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
	relay.CountEvents = append(relay.CountEvents, db.CountEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)

	bl := blossom.New(relay, "http://localhost:3334")
	bl.Store = blossom.EventStoreBlobIndexWrapper{Store: db, ServiceURL: bl.ServiceURL}
	bl.StoreBlob = append(bl.StoreBlob, func(ctx context.Context, sha256 string, body []byte) error {
		fmt.Println("storing", sha256, len(body))
		return nil
	})
	bl.LoadBlob = append(bl.LoadBlob, func(ctx context.Context, sha256 string) ([]byte, error) {
		fmt.Println("loading", sha256)
		return []byte("aaaaa"), nil
	})

	fmt.Println("running on :3334")
	http.ListenAndServe(":3334", relay)
}
