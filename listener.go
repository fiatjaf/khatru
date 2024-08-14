package khatru

import (
	"context"
	"errors"
	"slices"

	"github.com/nbd-wtf/go-nostr"
)

var ErrSubscriptionClosedByClient = errors.New("subscription closed by client")

type listenerSpec struct {
	id       string // kept here so we can easily match against it removeListenerId
	cancel   context.CancelCauseFunc
	index    int
	subrelay *Relay // this is important when we're dealing with routing, otherwise it will be always the same
}

type listener struct {
	id     string // duplicated here so we can easily send it on notifyListeners
	filter nostr.Filter
	ws     *WebSocket
}

func (rl *Relay) GetListeningFilters() []nostr.Filter {
	respfilters := make([]nostr.Filter, len(rl.listeners))
	for i, l := range rl.listeners {
		respfilters[i] = l.filter
	}
	return respfilters
}

// addListener may be called multiple times for each id and ws -- in which case each filter will
// be added as an independent listener
func (rl *Relay) addListener(
	ws *WebSocket,
	id string,
	subrelay *Relay,
	filter nostr.Filter,
	cancel context.CancelCauseFunc,
) {
	rl.clientsMutex.Lock()
	defer rl.clientsMutex.Unlock()

	if specs, ok := rl.clients[ws]; ok /* this will always be true unless client has disconnected very rapidly */ {
		idx := len(subrelay.listeners)
		rl.clients[ws] = append(specs, listenerSpec{
			id:       id,
			cancel:   cancel,
			subrelay: subrelay,
			index:    idx,
		})
		subrelay.listeners = append(subrelay.listeners, listener{
			ws:     ws,
			id:     id,
			filter: filter,
		})
	}
}

// remove a specific subscription id from listeners for a given ws client
// and cancel its specific context
func (rl *Relay) removeListenerId(ws *WebSocket, id string) {
	rl.clientsMutex.Lock()
	defer rl.clientsMutex.Unlock()

	if specs, ok := rl.clients[ws]; ok {
		// swap delete specs that match this id
		for s := len(specs) - 1; s >= 0; s-- {
			spec := specs[s]
			if spec.id == id {
				spec.cancel(ErrSubscriptionClosedByClient)
				specs[s] = specs[len(specs)-1]
				specs = specs[0 : len(specs)-1]
				rl.clients[ws] = specs

				// swap delete listeners one at a time, as they may be each in a different subrelay
				srl := spec.subrelay // == rl in normal cases, but different when this came from a route

				if spec.index != len(srl.listeners)-1 {
					movedFromIndex := len(srl.listeners) - 1
					moved := srl.listeners[movedFromIndex] // this wasn't removed, but will be moved
					srl.listeners[spec.index] = moved

					// now we must update the the listener we just moved
					// so its .index reflects its new position on srl.listeners
					movedSpecs := rl.clients[moved.ws]
					idx := slices.IndexFunc(movedSpecs, func(ls listenerSpec) bool {
						return ls.index == movedFromIndex && ls.subrelay == srl
					})
					movedSpecs[idx].index = spec.index
					rl.clients[moved.ws] = movedSpecs
				}
				srl.listeners = srl.listeners[0 : len(srl.listeners)-1] // finally reduce the slice length
			}
		}
	}
}

func (rl *Relay) removeClientAndListeners(ws *WebSocket) {
	rl.clientsMutex.Lock()
	defer rl.clientsMutex.Unlock()
	if specs, ok := rl.clients[ws]; ok {
		// swap delete listeners and delete client (all specs will be deleted)
		for s, spec := range specs {
			// no need to cancel contexts since they inherit from the main connection context
			// just delete the listeners (swap-delete)
			srl := spec.subrelay

			if spec.index != len(srl.listeners)-1 {
				movedFromIndex := len(srl.listeners) - 1
				moved := srl.listeners[movedFromIndex] // this wasn't removed, but will be moved
				srl.listeners[spec.index] = moved

				// temporarily update the spec of the listener being removed to have index == -1
				// (since it was removed) so it doesn't match in the search below
				rl.clients[ws][s].index = -1

				// now we must update the the listener we just moved
				// so its .index reflects its new position on srl.listeners
				movedSpecs := rl.clients[moved.ws]
				idx := slices.IndexFunc(movedSpecs, func(ls listenerSpec) bool {
					return ls.index == movedFromIndex && ls.subrelay == srl
				})
				movedSpecs[idx].index = spec.index
				rl.clients[moved.ws] = movedSpecs
			}
			srl.listeners = srl.listeners[0 : len(srl.listeners)-1] // finally reduce the slice length
		}
	}
	delete(rl.clients, ws)
}

func (rl *Relay) notifyListeners(event *nostr.Event) {
	for _, listener := range rl.listeners {
		if listener.filter.Matches(event) {
			for _, pb := range rl.PreventBroadcast {
				if pb(listener.ws, event) {
					return
				}
			}
			listener.ws.WriteJSON(nostr.EventEnvelope{SubscriptionID: &listener.id, Event: *event})
		}
	}
}
