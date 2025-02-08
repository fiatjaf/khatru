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
	_ = os.MkdirAll(db.Path, 0755)
	if err := db.Init(); err != nil {
		panic(err)
	}
	defer db.Close()

	relay := khatru.NewRelay(db)
	relay.WithCountEvents(db.CountEvents)

	fmt.Println("running on :3334")
	_ = http.ListenAndServe(":3334", relay)
}
