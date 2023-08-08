package main

import (
	"fmt"
	"net/http"

	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/plugins/storage/sqlite3"
)

func main() {
	relay := khatru.NewRelay()

	db := sqlite3.SQLite3Backend{DatabaseURL: "/tmp/khatru-sqlite-tmp"}
	if err := db.Init(); err != nil {
		panic(err)
	}

	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
	relay.CountEvents = append(relay.CountEvents, db.CountEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)

	fmt.Println("running on :3334")
	http.ListenAndServe(":3334", relay)
}
