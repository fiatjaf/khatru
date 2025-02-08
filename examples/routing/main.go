package main

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/fiatjaf/eventstore/slicestore"
	"github.com/fiatjaf/eventstore/sqlite3"
	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
)

func main() {
	router := khatru.NewRouter()

	{
		db := &slicestore.SliceStore{}
		_ = db.Init()
		relay := khatru.NewRelay(db)
		relay.WithCountEvents(db.CountEvents)

		router.Route().
			Req(func(filter nostr.Filter) bool { return slices.Contains(filter.Kinds, 30023) }).
			Event(func(event *nostr.Event) bool { return event.Kind == 30023 }).
			Relay(relay)
	}

	{
		db := &sqlite3.SQLite3Backend{DatabaseURL: "/tmp/t"}
		_ = db.Init()
		relay := khatru.NewRelay(db)
		relay.WithCountEvents(db.CountEvents)

		router.Route().
			Req(func(filter nostr.Filter) bool {
				return slices.Contains(filter.Kinds, 1) && slices.Contains(filter.Tags["t"], "spam")
			}).
			Event(func(event *nostr.Event) bool {
				return event.Kind == 1 && event.Tags.GetFirst([]string{"t", "spam"}) != nil
			}).
			Relay(relay)
	}

	{
		db := slicestore.SliceStore{}
		_ = db.Init()
		relay := khatru.NewRelay()
		relay.WithCountEvents(db.CountEvents)

		router.Route().
			Req(func(filter nostr.Filter) bool { return slices.Contains(filter.Kinds, 1) }).
			Event(func(event *nostr.Event) bool { return event.Kind == 1 }).
			Relay(relay)
	}

	fmt.Println("running on :3334")
	_ = http.ListenAndServe(":3334", router)
}
