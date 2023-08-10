package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/plugins/storage/lmdbn"
)

func main() {
	relay := khatru.NewRelay()

	db := lmdbn.LMDBBackend{Path: "/tmp/khatru-lmdbn-tmp"}
	os.MkdirAll(db.Path, 0755)
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
