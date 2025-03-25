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
	relay := khatru.NewRelay()

	db := lmdb.LMDBBackend{Path: "/tmp/exclusive"}
	os.MkdirAll(db.Path, 0o755)
	if err := db.Init(); err != nil {
		panic(err)
	}

	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
	relay.CountEvents = append(relay.CountEvents, db.CountEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)

	relay.RejectEvent = append(relay.RejectEvent, policies.PreventTooManyIndexableTags(10, nil, nil))
	relay.RejectFilter = append(relay.RejectFilter, policies.NoComplexFilters)

	relay.OnEventSaved = append(relay.OnEventSaved, func(ctx context.Context, event *nostr.Event) {
	})

	fmt.Println("running on :3334")
	http.ListenAndServe(":3334", relay)
}

func deleteStuffThatCanBeFoundElsewhere() {
}
