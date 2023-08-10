package main

import (
	"fmt"
	"net/http"

	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/plugins/storage/elasticsearch"
)

func main() {
	relay := khatru.NewRelay()

	db := elasticsearch.ElasticsearchStorage{URL: ""}
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
