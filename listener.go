package khatru

import (
	"github.com/nbd-wtf/go-nostr"
	"github.com/puzpuzpuz/xsync/v2"
)

type Listener struct {
	filters nostr.Filters
}

var listeners = xsync.NewTypedMapOf[*WebSocket, map[string]*Listener](pointerHasher[WebSocket])

func GetListeningFilters() nostr.Filters {
	respfilters := make(nostr.Filters, 0, listeners.Size()*2)

	// here we go through all the existing listeners
	listeners.Range(func(_ *WebSocket, subs map[string]*Listener) bool {
		for _, listener := range subs {
			for _, listenerfilter := range listener.filters {
				for _, respfilter := range respfilters {
					// check if this filter specifically is already added to respfilters
					if nostr.FilterEqual(listenerfilter, respfilter) {
						goto nextconn
					}
				}

				// field not yet present on respfilters, add it
				respfilters = append(respfilters, listenerfilter)

				// continue to the next filter
			nextconn:
				continue
			}
		}

		return true
	})

	// respfilters will be a slice with all the distinct filter we currently have active
	return respfilters
}

func setListener(id string, ws *WebSocket, filters nostr.Filters) {
	subs, _ := listeners.LoadOrCompute(ws, func() map[string]*Listener { return make(map[string]*Listener) })
	subs[id] = &Listener{filters: filters}
}

// Remove a specific subscription id from listeners for a given ws client
func removeListenerId(ws *WebSocket, id string) {
	if subs, ok := listeners.Load(ws); ok {
		delete(subs, id)
		if len(subs) == 0 {
			listeners.Delete(ws)
		}
	}
}

// Remove WebSocket conn from listeners
func removeListener(ws *WebSocket) {
	listeners.Delete(ws)
}

func notifyListeners(event *nostr.Event) {
	listeners.Range(func(ws *WebSocket, subs map[string]*Listener) bool {
		for id, listener := range subs {
			if !listener.filters.Match(event) {
				continue
			}
			ws.WriteJSON(nostr.EventEnvelope{SubscriptionID: &id, Event: *event})
		}
		return true
	})
}
