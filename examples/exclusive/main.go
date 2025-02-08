package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/fiatjaf/eventstore/lmdb"
	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/policies"
	"github.com/nbd-wtf/go-nostr"
)

func main() {
	db := &lmdb.LMDBBackend{Path: "/tmp/exclusive"}
	_ = os.MkdirAll(db.Path, 0755)
	if err := db.Init(); err != nil {
		panic(err)
	}
	defer db.Close()

	relay := khatru.NewRelay(db)
	relay.WithCountEvents(db.CountEvents)

	relay.RejectEvent = append(relay.RejectEvent, policies.PreventTooManyIndexableTags(10, nil, nil))
	relay.RejectFilter = append(relay.RejectFilter, policies.NoComplexFilters)
	relay.OnEventSaved = append(relay.OnEventSaved, func(ctx context.Context, event *nostr.Event) {})

	fmt.Println("running on :3334")
	_ = http.ListenAndServe(":3334", relay)
}
