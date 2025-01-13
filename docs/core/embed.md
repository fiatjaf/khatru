---
outline: deep
---

# Mixing a `khatru` relay with other HTTP handlers

If you already have a web server with all its HTML handlers or a JSON HTTP API or anything like that, something like:

```go
func main() {
	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("/.well-known/nostr.json", handleNIP05)
	mux.HandleFunc("/page/{page}", handlePage)
	mux.HandleFunc("/", handleHomePage)

	log.Printf("listening at http://0.0.0.0:8080")
	http.ListenAndServe("0.0.0.0:8080", mux)
}
```

Then you can easily inject a relay or two there in alternative paths if you want:

```diff
 	mux := http.NewServeMux()

+	relay1 := khatru.NewRelay()
+	relay2 := khatru.NewRelay()
+	// and so on

 	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
 	mux.HandleFunc("/.well-known/nostr.json", handleNIP05)
 	mux.HandleFunc("/page/{page}", handlePage)
 	mux.HandleFunc("/", handleHomePage)
+	mux.Handle("/relay1", relay1)
+	mux.Handle("/relay2", relay2)
+	// and so forth

 	log.Printf("listening at http://0.0.0.0:8080")
```

Imagine each of these relay handlers is different, each can be using a different eventstore and have different policies for writing and reading.

## Exposing a relay interface at the root

If you want to expose your relay at the root path `/` that is also possible. You can just use it as the `mux` directly:

```go
func main() {
	relay := khatru.NewRelay()
	// ... -- relay configuration steps (omitted for brevity)

	mux := relay.Router() // the relay comes with its own http.ServeMux inside

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("/.well-known/nostr.json", handleNIP05)
	mux.HandleFunc("/page/{page}", handlePage)
	mux.HandleFunc("/", handleHomePage)

	log.Printf("listening at http://0.0.0.0:8080")
	http.ListenAndServe("0.0.0.0:8080", mux)
}
```

Every [`khatru.Relay`](https://pkg.go.dev/github.com/fiatjaf/khatru#Relay) instance comes with its own ['http.ServeMux`](https://pkg.go.dev/net/http#ServeMux) inside. It ensures all requests are handled normally, but intercepts the requests that are pertinent to the relay operation, specifically the WebSocket requests, and the [NIP-11](https://nips.nostr.com/11) and the [NIP-86](https://nips.nostr.com/86) HTTP requests.

## Exposing multiple relays at the same path or at the root

That's also possible, as long as you have a way of differentiating each HTTP request that comes at the middleware level and associating it with a `khatru.Relay` instance in the background.

See [dynamic](../cookbook/dynamic) for an example that does that using the subdomain. [`countries`](https://git.fiatjaf.com/countries) does it using the requester country implied from its IP address.
