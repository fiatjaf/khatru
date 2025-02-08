package main

import (
	"fmt"
	"net/http"

	"github.com/fiatjaf/eventstore/elasticsearch"
	"github.com/fiatjaf/khatru"
)

func main() {
	db := &elasticsearch.ElasticsearchStorage{URL: ""}
	if err := db.Init(); err != nil {
		panic(err)
	}

	relay := khatru.NewRelay(db)
	relay.WithCountEvents(db.CountEvents)

	fmt.Println("running on :3334")
	if err := http.ListenAndServe(":3334", relay); err != nil {
		panic(err)
	}
}
