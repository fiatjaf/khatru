package khatru

import (
	"encoding/json"
	"net/http"
	"strings"
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

	// resolve relative icon and banner URLs against base URL
	baseURL := rl.getBaseURL(r)
	if info.Icon != "" && !strings.HasPrefix(info.Icon, "http://") && !strings.HasPrefix(info.Icon, "https://") {
		info.Icon = strings.TrimSuffix(baseURL, "/") + "/" + strings.TrimPrefix(info.Icon, "/")
	}
	if info.Banner != "" && !strings.HasPrefix(info.Banner, "http://") && !strings.HasPrefix(info.Banner, "https://") {
		info.Banner = strings.TrimSuffix(baseURL, "/") + "/" + strings.TrimPrefix(info.Banner, "/")
	}

	for _, ovw := range rl.OverwriteRelayInformation {
		info = ovw(r.Context(), r, info)
	}

	json.NewEncoder(w).Encode(info)
}
