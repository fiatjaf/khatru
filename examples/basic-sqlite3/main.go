package main

import (
	"fmt"
	"net/http"

	"github.com/fiatjaf/eventstore/sqlite3"
	"github.com/fiatjaf/khatru"
)

func main() {
	db := &sqlite3.SQLite3Backend{DatabaseURL: "/tmp/khatru-sqlite-tmp"}
	if err := db.Init(); err != nil {
		panic(err)
	}
	defer db.Close()

	relay := khatru.NewRelay(db)
	relay.WithCountEvents(db.CountEvents)

	fmt.Println("running on :3334")
	_ = http.ListenAndServe(":3334", relay)
}
