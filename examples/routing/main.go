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
	var r1, r2, r3 *khatru.Relay

	{
		db := &slicestore.SliceStore{}
		_ = db.Init()
		r1 = khatru.NewRelay(db)
		r1.WithCountEvents(db.CountEvents)
	}

	{
		db := &sqlite3.SQLite3Backend{DatabaseURL: "/tmp/t"}
		_ = db.Init()
		r2 = khatru.NewRelay(db)
		r2.WithCountEvents(db.CountEvents)
	}

	{
		db := slicestore.SliceStore{}
		_ = db.Init()
		r3 = khatru.NewRelay()
		r3.WithCountEvents(db.CountEvents)
	}

	router := khatru.NewRouter()

	router.Route().
		Req(func(filter nostr.Filter) bool {
			return slices.Contains(filter.Kinds, 30023)
		}).
		Event(func(event *nostr.Event) bool {
			return event.Kind == 30023
		}).
		Relay(r1)

	router.Route().
		Req(func(filter nostr.Filter) bool {
			return slices.Contains(filter.Kinds, 1) && slices.Contains(filter.Tags["t"], "spam")
		}).
		Event(func(event *nostr.Event) bool {
			return event.Kind == 1 && event.Tags.GetFirst([]string{"t", "spam"}) != nil
		}).
		Relay(r2)

	router.Route().
		Req(func(filter nostr.Filter) bool {
			return slices.Contains(filter.Kinds, 1)
		}).
		Event(func(event *nostr.Event) bool {
			return event.Kind == 1
		}).
		Relay(r3)

	fmt.Println("running on :3334")
	http.ListenAndServe(":3334", router)
}
