---
layout: home

hero:
  name: khatru
  text: a framework for making Nostr relays
  tagline: write your custom relay with code over configuration
  actions:
    - theme: brand
      text: Get Started
      link: /getting-started

features:
  - title: It's a library
    icon: 🐢
    link: /getting-started
    details: This is not an executable that you have to tweak with config files, it's a library that you import and use, so you just write code and it does exactly what you want.
  - title: It's very very customizable
    icon: 🎶
    link: /core/embed
    details: Run arbitrary functions to reject events, reject filters, overwrite results of queries, perform actual queries, mix the relay stuff with other HTTP handlers or even run it inside an existing website.
  - title: It plugs into event stores easily
    icon: 📦
    link: /core/eventstore
    details: khatru's companion, the `eventstore` library, provides all methods for storing and querying events efficiently from SQLite, LMDB, Postgres, Badger and others.
  - title: It supports NIP-42 AUTH
    icon: 🪪
    link: /core/auth
    details: You can check if a client is authenticated or request AUTH anytime, or reject an event or a filter with an "auth-required:" and it will be handled automatically.
  - title: It supports NIP-86 Management API
    icon: 🛠️
    link: /core/management
    details: You just define your custom handlers for each RPC call and they will be exposed appropriately to management clients.
  - title: It's written in Go
    icon: 🛵
    link: https://pkg.go.dev/github.com/fiatjaf/khatru
    details: That means it is fast and lightweight, you can learn the language in 5 minutes and it builds your relay into a single binary that's easy to ship and deploy.
---

## A glimpse of `khatru`'s power

It allows you to create a fully-functional relay in 7 lines of code:

```go
func main() {
	relay := khatru.NewRelay()
	db := badger.BadgerBackend{Path: "/tmp/khatru-badgern-tmp"}
    db.Init()
	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)
	relay.ReplaceEvent = append(relay.ReplaceEvent, db.ReplaceEvent)
	http.ListenAndServe(":3334", relay)
}
```

After that you can customize it in infinite ways. See the links above.
