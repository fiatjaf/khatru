# khatru, a relay framework [![docs badge](https://img.shields.io/badge/docs-reference-blue)](https://pkg.go.dev/github.com/fiatjaf/khatru#Relay)

`author: pablof7z`

[![Run Tests](https://github.com/fiatjaf/khatru/actions/workflows/test.yml/badge.svg)](https://github.com/fiatjaf/khatru/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/fiatjaf/khatru.svg)](https://pkg.go.dev/github.com/fiatjaf/khatru)
[![Go Report Card](https://goreportcard.com/badge/github.com/fiatjaf/khatru)](https://goreportcard.com/report/github.com/fiatjaf/khatru)

Khatru makes it easy to write very very custom relays:

  - custom event or filter acceptance policies
  - custom `AUTH` handlers
  - custom storage and pluggable databases
  - custom webpages and other HTTP handlers

Here's a sample:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
)

func main() {
	// create the relay instance
	relay := khatru.NewRelay()

	// set up some basic properties (will be returned on the NIP-11 endpoint)
	relay.Info.Name = "my relay"
	relay.Info.PubKey = "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	relay.Info.Description = "this is my custom relay"
	relay.Info.IconURL = "https://external-content.duckduckgo.com/iu/?u=https%3A%2F%2Fliquipedia.net%2Fcommons%2Fimages%2F3%2F35%2FSCProbe.jpg&f=1&nofb=1&ipt=0cbbfef25bce41da63d910e86c3c343e6c3b9d63194ca9755351bb7c2efa3359&ipo=images"

	// you must bring your own storage scheme -- if you want to have any
	store := make(map[string]*nostr.Event, 120)

	// set up the basic relay functions
	relay.StoreEvent = append(relay.StoreEvent,
		func(ctx context.Context, event *nostr.Event) error {
			store[event.ID] = event
			return nil
		},
	)
	relay.QueryEvents = append(relay.QueryEvents,
		func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
			ch := make(chan *nostr.Event)
			go func() {
				for _, evt := range store {
					if filter.Matches(evt) {
						ch <- evt
					}
				}
				close(ch)
			}()
			return ch, nil
		},
	)
	relay.DeleteEvent = append(relay.DeleteEvent,
		func(ctx context.Context, event *nostr.Event) error {
			delete(store, event.ID)
			return nil
		},
	)

	// there are many other configurable things you can set
	relay.RejectEvent = append(relay.RejectEvent,
		func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
			if event.PubKey == "fa984bd7dbb282f07e16e7ae87b26a2a7b9b90b7246a44771f0cf5ae58018f52" {
				return true, "we don't allow this person to write here"
			}
			return false, "" // anyone else can
		},
	)
	relay.OnConnect = append(relay.OnConnect,
		func(ctx context.Context) {
			// request NIP-42 AUTH from everybody
			relay.RequestAuth(ctx)
		},
	)
	relay.OnAuth = append(relay.OnAuth,
		func(ctx context.Context, pubkey string) {
			// and when they auth we just log that for nothing
			log.Println(pubkey + " is authed!")
		},
	)
	// check the docs for more goodies!

	mux := relay.Router()
	// set up other http handlers
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/html")
		fmt.Fprintf(w, `<b>welcome</b> to my relay!`)
	})

	// start the server
	fmt.Println("running on :3334")
	http.ListenAndServe(":3334", relay)
}
```

### But I don't want to write my own database!

Fear no more. Using the https://github.com/fiatjaf/eventstore module you get a bunch of compatible databases out of the box and you can just plug them into your relay. For example, [sqlite](https://pkg.go.dev/github.com/fiatjaf/eventstore/sqlite3):

```go
	db := sqlite3.SQLite3Backend{DatabaseURL: "/tmp/khatru-sqlite-tmp"}
	if err := db.Init(); err != nil {
		panic(err)
	}

	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
	relay.CountEvents = append(relay.CountEvents, db.CountEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)
```
