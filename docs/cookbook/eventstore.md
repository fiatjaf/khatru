---
outline: deep
---

# Using the `eventstore` library

The [`eventstore`](https://github.com/fiatjaf/eventstore) library has adapters that you can easily plug into `khatru`'s:

* `StoreEvent`
* `DeleteEvent`
* `QueryEvents`
* `CountEvents`

For all of them you start by instantiating a struct containing some basic options and a pointer (a file path for local databases, a connection string for remote databases) to the data. Then you call `.Init()` and if all is well you're ready to start storing, querying and deleting events, so you can pass the respective functions to their `khatru` counterparts. These eventstores also expose a `.Close()` function that must be called if you're going to stop using that store and keep your application open.

Here's an example with the [Badger](https://pkg.go.dev/github.com/fiatjaf/eventstore/badger) adapter, made for the [Badger](https://github.com/dgraph-io/badger) embedded key-value database:

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/fiatjaf/eventstore/badger"
	"github.com/fiatjaf/khatru"
)

func main() {
	relay := khatru.NewRelay()

	db := badger.BadgerBackend{Path: "/tmp/khatru-badger-tmp"}
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
```

[LMDB](https://pkg.go.dev/github.com/fiatjaf/eventstore/lmdb) works the same way.

[SQLite](https://pkg.go.dev/github.com/fiatjaf/eventstore/sqlite3) also stores things locally so it only needs a `Path`.

[PostgreSQL](https://pkg.go.dev/github.com/fiatjaf/eventstore/postgresql) and [MySQL](https://pkg.go.dev/github.com/fiatjaf/eventstore/mysql) use remote connections to database servers, so they take a `DatabaseURL` parameter, but after that it's the same.

## Using two at a time

If you want to use two different adapters at the same time that's easy. Just add both to the corresponding slices:

```go
	relay.StoreEvent = append(relay.StoreEvent, db1.SaveEvent, db2.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, db1.QueryEvents, db2.SaveEvent)
```

But that will duplicate events on both and then return duplicated events on each query.

## Sharding

You can do a kind of sharding, for example, by storing some events in one store and others in another:

For example, maybe you want kind 1 events in `db1` and kind 30023 events in `db30023`:

```go
	relay.StoreEvent = append(relay.StoreEvent, func (ctx context.Context, evt *nostr.Event) error {
		switch evt.Kind {
		case 1:
			return db1.StoreEvent(ctx, evt)
		case 30023:
			return db30023.StoreEvent(ctx, evt)
		default:
			return nil
		}
	})
	relay.QueryEvents = append(relay.QueryEvents, func (ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
		for _, kind := range filter.Kinds {
			switch kind {
			case 1:
				filter1 := filter
				filter1.Kinds = []int{1}
				return db1.QueryEvents(ctx, filter1)
			case 30023:
				filter30023 := filter
				filter30023.Kinds = []int{30023}
				return db30023.QueryEvents(ctx, filter30023)
			default:
				return nil, nil
			}
		}
	})
```

## Search

See [search](search).
