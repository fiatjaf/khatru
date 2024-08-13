package khatru

import (
	"context"
	"errors"
	"slices"

	"github.com/nbd-wtf/go-nostr"
)

var ErrSubscriptionClosedByClient = errors.New("subscription closed by client")

type listenerSpec struct {
	id       string
	cancel   context.CancelCauseFunc
	index    int
	subrelay *Relay
}

type listener struct {
	id     string
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

func (rl *Relay) addListener(
	ws *WebSocket,
	id string,
	subrelay *Relay,
	filter nostr.Filter,
	cancel context.CancelCauseFunc,
) {
	rl.clientsMutex.Lock()
	defer rl.clientsMutex.Unlock()

	if specs, ok := rl.clients[ws]; ok {
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
		for s := len(specs) - 1; s >= 0; s-- {
			spec := specs[s]
			if spec.id == id {
				spec.cancel(ErrSubscriptionClosedByClient)

				// swap delete listeners one at a time, as they may be each in a different subrelay
				srl := spec.subrelay // == rl in normal cases, but different when this came from a route
				lastIndex := len(srl.listeners) - 1

				if spec.index >= 0 && spec.index < len(srl.listeners) {
					if spec.index != lastIndex {
						moved := srl.listeners[lastIndex]
						srl.listeners[spec.index] = moved

						movedSpecs := rl.clients[moved.ws]
						idx := slices.IndexFunc(movedSpecs, func(ls listenerSpec) bool {
							return ls.index == lastIndex
						})
						if idx != -1 {
							movedSpecs[idx].index = spec.index
						}
					}
					srl.listeners = srl.listeners[:lastIndex]
				}

				specs[s] = specs[len(specs)-1]
				specs = specs[:len(specs)-1]
				rl.clients[ws] = specs
			}
		}
	}
}

func (rl *Relay) removeClientAndListeners(ws *WebSocket) {
	rl.clientsMutex.Lock()
	defer rl.clientsMutex.Unlock()
	if specs, ok := rl.clients[ws]; ok {
		// swap delete listeners and delete client (all specs will be deleted)
		for s := len(specs) - 1; s >= 0; s-- {
			// no need to cancel contexts since they inherit from the main connection context
			// just delete the listeners (swap-delete)
			spec := specs[s]
			srl := spec.subrelay
			lastIndex := len(srl.listeners) - 1

			if spec.index >= 0 && spec.index < len(srl.listeners) {
				if spec.index != lastIndex {
					moved := srl.listeners[lastIndex] // this wasn't removed, but will be moved
					srl.listeners[spec.index] = moved

					movedSpecs := rl.clients[moved.ws]
					idx := slices.IndexFunc(movedSpecs, func(ls listenerSpec) bool {
						return ls.index == lastIndex
					})
					if idx != -1 {
						movedSpecs[idx].index = spec.index
					}
				}
				srl.listeners = srl.listeners[:lastIndex]
			}

			specs[s] = specs[len(specs)-1]
			specs = specs[:len(specs)-1]
			rl.clients[ws] = specs
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
