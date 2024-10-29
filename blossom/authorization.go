package blossom

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

func readAuthorization(r *http.Request) (*nostr.Event, error) {
	token := r.Header.Get("Authorization")
	if !strings.HasPrefix(token, "Nostr ") {
		return nil, nil
	}

	var reader io.Reader
	reader = bytes.NewReader([]byte(token)[6:])
	reader = base64.NewDecoder(base64.StdEncoding, reader)
	var evt nostr.Event
	err := json.NewDecoder(reader).Decode(&evt)

	if err != nil || evt.Kind != 24242 || len(evt.ID) != 64 || !evt.CheckID() {
		return nil, fmt.Errorf("invalid event")
	}

	if ok, _ := evt.CheckSignature(); !ok {
		return nil, fmt.Errorf("invalid signature")
	}

	expirationTag := evt.Tags.GetFirst([]string{"expiration", ""})
	if expirationTag == nil {
		return nil, fmt.Errorf("missing \"expiration\" tag")
	}
	expiration, _ := strconv.ParseInt((*expirationTag)[1], 10, 64)
	if nostr.Timestamp(expiration) < nostr.Now() {
		return nil, fmt.Errorf("event expired")
	}

	return &evt, nil
}
