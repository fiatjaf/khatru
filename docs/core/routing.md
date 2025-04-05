---
outline: deep
---

# Request Routing

If you have one (or more) set of policies that have to be executed in sequence (for example, first you check for the presence of a tag, then later in the next policies you use that tag without checking) and they only apply to some class of events, but you still want your relay to deal with other classes of events that can lead to cumbersome sets of rules, always having to check if an event meets the requirements and so on. There is where routing can help you.

It also can be handy if you get a [`khatru.Relay`](https://pkg.go.dev/github.com/fiatjaf/khatru#Relay) from somewhere else, like a library such as [`relay29`](https://github.com/fiatjaf/relay29), and you want to combine it with other policies without some interfering with the others. As in the example below:

```go
sk := os.Getenv("RELAY_SECRET_KEY")

// a relay for NIP-29 groups
groupsStore := badger.BadgerBackend{}
groupsStore.Init()
groupsRelay, _ := khatru29.Init(relay29.Options{Domain: "example.com", DB: groupsStore, SecretKey: sk})
// ...

// a relay for everything else
publicStore := slicestore.SliceStore{}
publicStore.Init()
publicRelay := khatru.NewRelay()
publicRelay.StoreEvent = append(publicRelay.StoreEvent, publicStore.SaveEvent)
publicRelay.QueryEvents = append(publicRelay.QueryEvents, publicStore.QueryEvents)
publicRelay.CountEvents = append(publicRelay.CountEvents, publicStore.CountEvents)
publicRelay.DeleteEvent = append(publicRelay.DeleteEvent, publicStore.DeleteEvent)
// ...

// a higher-level relay that just routes between the two above
router := khatru.NewRouter()

// route requests and events to the groups relay
router.Route().
	Req(func (filter nostr.Filter) bool {
		_, hasHTag := filter.Tags["h"]
		if hasHTag {
			return true
		}
		return slices.Contains(filter.Kinds, func (k int) bool { return k == 39000 || k == 39001 || k == 39002 })
	}).
	Event(func (event *nostr.Event) bool {
		switch {
		case event.Kind <= 9021 && event.Kind >= 9000:
			return true
		case event.Kind <= 39010 && event.Kind >= 39000:
			return true
		case event.Kind <= 12 && event.Kind >= 9:
			return true
		case event.Tags.Find("h") != nil:
			return true
		default:
			return false
		}
	}).
	Relay(groupsRelay)

// route requests and events to the other
router.Route().
	Req(func (filter nostr.Filter) bool { return true }).
	Event(func (event *nostr.Event) bool { return true }).
	Relay(publicRelay)
```
