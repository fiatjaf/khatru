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
	db1 := slicestore.SliceStore{}
	db1.Init()
	r1 := khatru.NewRelay()
	r1.StoreEvent = append(r1.StoreEvent, db1.SaveEvent)
	r1.QueryEvents = append(r1.QueryEvents, db1.QueryEvents)
	r1.CountEvents = append(r1.CountEvents, db1.CountEvents)
	r1.DeleteEvent = append(r1.DeleteEvent, db1.DeleteEvent)

	db2 := sqlite3.SQLite3Backend{DatabaseURL: "/tmp/t"}
	db2.Init()
	r2 := khatru.NewRelay()
	r2.StoreEvent = append(r2.StoreEvent, db2.SaveEvent)
	r2.QueryEvents = append(r2.QueryEvents, db2.QueryEvents)
	r2.CountEvents = append(r2.CountEvents, db2.CountEvents)
	r2.DeleteEvent = append(r2.DeleteEvent, db2.DeleteEvent)

	db3 := slicestore.SliceStore{}
	db3.Init()
	r3 := khatru.NewRelay()
	r3.StoreEvent = append(r3.StoreEvent, db3.SaveEvent)
	r3.QueryEvents = append(r3.QueryEvents, db3.QueryEvents)
	r3.CountEvents = append(r3.CountEvents, db3.CountEvents)
	r3.DeleteEvent = append(r3.DeleteEvent, db3.DeleteEvent)

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
			return event.Kind == 1 && event.Tags.FindWithValue("t", "spam") != nil
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
