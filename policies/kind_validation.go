package policies

import (
	"context"
	"encoding/json"

	"github.com/nbd-wtf/go-nostr"
)

func ValidateKind(ctx context.Context, evt *nostr.Event) (bool, string) {
	switch evt.Kind {
	case 0:
		var m struct {
			Name string `json:"name"`
		}
		json.Unmarshal([]byte(evt.Content), &m)
		if m.Name == "" {
			return true, "missing json name in kind 0"
		}
	case 1:
		return false, ""
	case 2:
		return true, "this kind has been deprecated"
	}

	// TODO: all other kinds

	return false, ""
}
