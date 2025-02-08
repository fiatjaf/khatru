package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/fiatjaf/eventstore/lmdb"
	"github.com/fiatjaf/khatru"
)

func main() {
	db := &lmdb.LMDBBackend{Path: "/tmp/khatru-lmdb-tmp"}
	os.MkdirAll(db.Path, 0755)
	if err := db.Init(); err != nil {
		panic(err)
	}

	relay := khatru.NewRelay()
	relay.WithCountEvents(db.CountEvents)

	fmt.Println("running on :3334")
	http.ListenAndServe(":3334", relay)
}
