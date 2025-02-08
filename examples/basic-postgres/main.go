package main

import (
	"fmt"
	"net/http"

	"github.com/fiatjaf/eventstore/postgresql"
	"github.com/fiatjaf/khatru"
)

func main() {
	db := &postgresql.PostgresBackend{DatabaseURL: "postgresql://localhost:5432/tmp-khatru-relay"}
	if err := db.Init(); err != nil {
		panic(err)
	}
	defer db.Close()

	relay := khatru.NewRelay(db)
	relay.WithCountEvents(db.CountEvents)

	fmt.Println("running on :3334")
	_ = http.ListenAndServe(":3334", relay)
}
