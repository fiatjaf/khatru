package khatru

import (
	"encoding/json"
	"net/http"
)

func (rl *Relay) HandleNIP11(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/nostr+json")

	info := *rl.Info

	if len(rl.DeleteEvent) > 0 {
		info.AddSupportedNIP(9)
	}
	if len(rl.CountEvents) > 0 {
		info.AddSupportedNIP(45)
	}
	if rl.Negentropy {
		info.AddSupportedNIP(77)
	}

	for _, ovw := range rl.OverwriteRelayInformation {
		info = ovw(r.Context(), r, info)
	}

	json.NewEncoder(w).Encode(info)
}
