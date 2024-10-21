---
outline: deep
---

# Getting Started

Download the library:

```bash
go get github.com/fiatjaf/khatru
```

Include the library:

```go
import "github.com/fiatjaf/khatru"
```

Then in your `main()` function, instantiate a new `Relay`:

```go
relay := khatru.NewRelay()
```

Optionally, set up basic info about the relay that will be returned according to [NIP-11](https://nips.nostr.com/11):

```go
relay.Info.Name = "my relay"
relay.Info.PubKey = "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
relay.Info.Description = "this is my custom relay"
relay.Info.Icon = "https://external-content.duckduckgo.com/iu/?u=https%3A%2F%2Fliquipedia.net%2Fcommons%2Fimages%2F3%2F35%2FSCProbe.jpg&f=1&nofb=1&ipt=0cbbfef25bce41da63d910e86c3c343e6c3b9d63194ca9755351bb7c2efa3359&ipo=images"
```

Now we must set up the basic functions for accepting events and answering queries. We could make our own querying engine from scratch, but we can also use [eventstore](https://github.com/fiatjaf/eventstore). In this example we'll use the SQLite adapter:

```go
db := sqlite3.SQLite3Backend{DatabaseURL: "/tmp/khatru-sqlite-tmp"}
if err := db.Init(); err != nil {
	panic(err)
}

relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)
relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)
```

These are lists of functions that will be called in order every time an `EVENT` is received, or a `REQ` query is received. You can add more than one handler there, you can have a function that reads from some other server, but just in some cases, you can do anything.

The next step is adding some protection, because maybe we don't want to allow _anyone_ to write to our relay. Maybe we want to only allow people that have a pubkey starting with `"a"`, `"b"` or `"c"`:

```go
relay.RejectEvent = append(relay.RejectEvent, func (ctx context.Context, event *nostr.Event) (reject bool, msg string) {
	firstHexChar := event.PubKey[0:1]
	if firstHexChar == "a" || firstHexChar == "b" || firstHexChar == "c" {
		return false, "" // allow
	}
	return true, "you're not allowed in this shard"
})
```

We can also make use of some default policies that come bundled with Khatru:

```go
import "github.com/fiatjaf/khatru" // implied

relay.RejectEvent = append(relay.RejectEvent, policies.PreventLargeTags(120), policies.PreventTimestampsInThePast(time.Hour * 2), policies.PreventTimestampsInTheFuture(time.Minute * 30))
```

There are many other ways to customize the relay behavior. Take a look at the [`Relay` struct docs](https://pkg.go.dev/github.com/fiatjaf/khatru#Relay) for more, or see the [cookbook](/cookbook/).

The last step is actually running the server. Our relay is actually an `http.Handler`, so it can just be ran directly with `http.ListenAndServe()` from the standard library:

```go
fmt.Println("running on :3334")
http.ListenAndServe(":3334", relay)
```

And that's it.
