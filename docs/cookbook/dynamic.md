---
outline: deep
---

# Generating `khatru` relays dynamically and serving them from the same path

Suppose you want to expose a different relay interface depending on the subdomain that is accessed. I don't know, maybe you want to serve just events with pictures on `pictures.example.com` and just events with audio files on `audios.example.com`; maybe you want just events in English on `en.example.com` and just examples in Portuguese on `pt.example.com`, there are many possibilities.

You could achieve that with a scheme like the following

```go
var topLevelHost = "example.com"
var mainRelay = khatru.NewRelay() // we're omitting all the configuration steps for brevity
var subRelays = xsync.NewMapOf[string, *khatru.Relay]()

func main () {
	handler := http.HandlerFunc(dynamicRelayHandler)

	log.Printf("listening at http://0.0.0.0:8080")
	http.ListenAndServe("0.0.0.0:8080", handler)
}

func dynamicRelayHandler(w http.ResponseWriter, r *http.Request) {
	var relay *khatru.Relay
	subdomain := r.Host[0 : len(topLevelHost)-len(topLevelHost)]
	if subdomain == "" {
		// no subdomain, use the main top-level relay
		relay = mainRelay
	} else {
		// call on subdomain, so get a dynamic relay
		subdomain = subdomain[0 : len(subdomain)-1] // remove dangling "."
		// get a dynamic relay
		relay, _ = subRelays.LoadOrCompute(subdomain, func () *khatru.Relay {
			return makeNewRelay(subdomain)
		})
	}

	relay.ServeHTTP(w, r)
}

func makeNewRelay (subdomain string) *khatru.Relay {
	// somehow use the subdomain to generate a relay with specific configurations
	relay := khatru.NewRelay()
	switch subdomain {
	case "pictures":
		// relay configuration shenanigans go here
	case "audios":
		// relay configuration shenanigans go here
	case "en":
		// relay configuration shenanigans go here
	case "pt":
		// relay configuration shenanigans go here
	}
	return relay
}
```

In practice you could come up with a way that allows all these dynamic relays to share a common underlying datastore, but this is out of the scope of this example.
