package khatru

import (
	"github.com/nbd-wtf/go-nostr"
)

// BroadcastEvent emits an event to all listeners whose filters' match, skipping all filters and actions
// it also doesn't attempt to store the event or trigger any reactions or callbacks
func (rl *Relay) BroadcastEvent(evt *nostr.Event) int {
	return rl.notifyListeners(evt)
}
