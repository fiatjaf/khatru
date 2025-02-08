package main

import (
	"fmt"
	"net/http"

	"github.com/fiatjaf/eventstore/badger"
	"github.com/fiatjaf/khatru"
)

func main() {
	db := &badger.BadgerBackend{Path: "/tmp/khatru-badgern-tmp"}
	if err := db.Init(); err != nil {
		panic(err)
	}
	defer db.Close()

	relay := khatru.NewRelay(db)
	relay.WithNegentropy()
	relay.WithCountEvents(db.CountEvents)

	fmt.Println("running on :3334")
	if err := http.ListenAndServe(":3334", relay); err != nil {
		panic(err)
	}
}
