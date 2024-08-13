package khatru

import (
	"math/rand"
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"github.com/stretchr/testify/require"
)

func FuzzRandomListenerClientRemoving(f *testing.F) {
	f.Add(uint(20), uint(20), uint(1))
	f.Fuzz(func(t *testing.T, utw uint, ubs uint, ualf uint) {
		totalWebsockets := int(utw)
		baseSubs := int(ubs)
		addListenerFreq := int(ualf) + 1

		rl := NewRelay()

		f := nostr.Filter{Kinds: []int{1}}
		cancel := func(cause error) {}

		websockets := make([]*WebSocket, 0, totalWebsockets*baseSubs)

		l := 0

		for i := 0; i < totalWebsockets; i++ {
			ws := &WebSocket{}
			websockets = append(websockets, ws)
			rl.clients[ws] = nil
		}

		s := 0
		for j := 0; j < baseSubs; j++ {
			for i := 0; i < totalWebsockets; i++ {
				ws := websockets[i]
				w := idFromSeqUpper(i)

				if s%addListenerFreq == 0 {
					l++
					rl.addListener(ws, w+":"+idFromSeqLower(j), rl, f, cancel)
				}

				s++
			}
		}

		require.Len(t, rl.clients, totalWebsockets)
		require.Len(t, rl.listeners, l)

		for ws := range rl.clients {
			rl.removeClientAndListeners(ws)
		}

		require.Len(t, rl.clients, 0)
		require.Len(t, rl.listeners, 0)
	})
}

func FuzzRandomListenerIdRemoving(f *testing.F) {
	f.Add(uint(20), uint(20), uint(1), uint(4))
	f.Fuzz(func(t *testing.T, utw uint, ubs uint, ualf uint, ualef uint) {
		totalWebsockets := int(utw)
		baseSubs := int(ubs)
		addListenerFreq := int(ualf) + 1
		addExtraListenerFreq := int(ualef) + 1

		if totalWebsockets > 1024 || baseSubs > 1024 {
			return
		}

		rl := NewRelay()

		f := nostr.Filter{Kinds: []int{1}}
		cancel := func(cause error) {}
		websockets := make([]*WebSocket, 0, totalWebsockets)

		type wsid struct {
			ws *WebSocket
			id string
		}

		subs := make([]wsid, 0, totalWebsockets*baseSubs)
		extra := 0

		for i := 0; i < totalWebsockets; i++ {
			ws := &WebSocket{}
			websockets = append(websockets, ws)
			rl.clients[ws] = nil
		}

		s := 0
		for j := 0; j < baseSubs; j++ {
			for i := 0; i < totalWebsockets; i++ {
				ws := websockets[i]
				w := idFromSeqUpper(i)

				if s%addListenerFreq == 0 {
					id := w + ":" + idFromSeqLower(j)
					rl.addListener(ws, id, rl, f, cancel)
					subs = append(subs, wsid{ws, id})

					if s%addExtraListenerFreq == 0 {
						rl.addListener(ws, id, rl, f, cancel)
						extra++
					}
				}

				s++
			}
		}

		require.Len(t, rl.clients, totalWebsockets)
		require.Len(t, rl.listeners, len(subs)+extra)

		rand.Shuffle(len(subs), func(i, j int) {
			subs[i], subs[j] = subs[j], subs[i]
		})
		for _, wsidToRemove := range subs {
			rl.removeListenerId(wsidToRemove.ws, wsidToRemove.id)
		}

		require.Len(t, rl.listeners, 0)
		require.Len(t, rl.clients, totalWebsockets)
		for _, specs := range rl.clients {
			require.Len(t, specs, 0)
		}
	})
}
