package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fiatjaf/eventstore/badger"
	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/blossom"
)

func main() {
	db := &badger.BadgerBackend{Path: "/tmp/khatru-badger-blossom-tmp"}
	if err := db.Init(); err != nil {
		panic(err)
	}
	defer db.Close()

	relay := khatru.NewRelay(db)
	relay.WithCountEvents(db.CountEvents)

	bl := blossom.New(relay, "http://localhost:3334")
	bl.Store = blossom.EventStoreBlobIndexWrapper{Store: db, ServiceURL: bl.ServiceURL}
	bl.StoreBlob = append(bl.StoreBlob, func(ctx context.Context, sha256 string, body []byte) error {
		fmt.Println("storing", sha256, len(body))
		return nil
	})
	bl.LoadBlob = append(bl.LoadBlob, func(ctx context.Context, sha256 string) (io.ReadSeeker, error) {
		fmt.Println("loading", sha256)
		blob := strings.NewReader("aaaaa")
		return blob, nil
	})

	fmt.Println("running on :3334")
	_ = http.ListenAndServe(":3334", relay)
}
