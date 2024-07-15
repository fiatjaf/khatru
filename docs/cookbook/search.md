---
outline: deep
---

# Implementing NIP-50 `search` support

The [`nostr.Filter` type](https://pkg.go.dev/github.com/nbd-wtf/go-nostr#Filter) has a `Search` field, so you basically just has to handle that if it's present.

It can be tricky to implement fulltext search properly though, so some [eventstores](eventstore) implement it natively, such as [Bluge](https://pkg.go.dev/github.com/fiatjaf/eventstore/bluge), [OpenSearch](https://pkg.go.dev/github.com/fiatjaf/eventstore/opensearch) and [ElasticSearch](https://pkg.go.dev/github.com/fiatjaf/eventstore/elasticsearch) (although for the last two you'll need an instance of these database servers running, while with Bluge it's embedded).

If you have any of these you can just use them just like any other eventstore:

```go
func main () {
    // other stuff here

	normal := &lmdb.LMDBBackend{Path: "data"}
	os.MkdirAll(normal.Path, 0755)
	if err := normal.Init(); err != nil {
		panic(err)
	}

	search := bluge.BlugeBackend{Path: "search", RawEventStore: normal}
	if err := search.Init(); err != nil {
		panic(err)
	}

	relay.StoreEvent = append(relay.StoreEvent, normal.SaveEvent, search.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, normal.QueryEvents, search.QueryEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, normal.DeleteEvent, search.DeleteEvent)

    // other stuff here
}
```

Note that in this case we're using the [LMDB](https://pkg.go.dev/github.com/fiatjaf/eventstore/lmdb) adapter for normal queries and it explicitly rejects any filter that contains a `Search` field, while [Bluge](https://pkg.go.dev/github.com/fiatjaf/eventstore/bluge) rejects any filter _without_ a `Search` value, which make them pair well together.

Other adapters, like [SQLite](https://pkg.go.dev/github.com/fiatjaf/eventstore/sqlite3), implement search functionality on their own, so if you don't want to use that you would have to have a middleware between, like:

```go
	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent, search.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, func (ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
        if len(filter.Search) > 0 {
			return search.QueryEvents(ctx, filter)
        } else {
			filterNoSearch := filter
            filterNoSearch.Search = ""
			return normal.QueryEvents(ctx, filterNoSearch)
		}
	})
```
