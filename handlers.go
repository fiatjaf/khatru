package khatru

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip11"
	"github.com/nbd-wtf/go-nostr/nip42"
)

// ServeHTTP implements http.Handler interface.
func (rl *Relay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") == "websocket" {
		rl.HandleWebsocket(w, r)
	} else if r.Header.Get("Accept") == "application/nostr+json" {
		rl.HandleNIP11(w, r)
	} else {
		rl.serveMux.ServeHTTP(w, r)
	}
}

func (rl *Relay) HandleWebsocket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	conn, err := rl.upgrader.Upgrade(w, r, nil)
	if err != nil {
		rl.Log.Printf("failed to upgrade websocket: %v\n", err)
		return
	}
	rl.clients.Store(conn, struct{}{})
	ticker := time.NewTicker(rl.PingPeriod)

	// NIP-42 challenge
	challenge := make([]byte, 8)
	rand.Read(challenge)

	ws := &WebSocket{
		conn:           conn,
		Challenge:      hex.EncodeToString(challenge),
		WaitingForAuth: make(chan struct{}),
	}

	// reader
	go func() {
		defer func() {
			ticker.Stop()
			if _, ok := rl.clients.Load(conn); ok {
				conn.Close()
				rl.clients.Delete(conn)
				removeListener(ws)
			}
		}()

		conn.SetReadLimit(rl.MaxMessageSize)
		conn.SetReadDeadline(time.Now().Add(rl.PongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(rl.PongWait))
			return nil
		})

		for _, onconnect := range rl.OnConnect {
			onconnect(ctx)
		}

		for {
			typ, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(
					err,
					websocket.CloseGoingAway,        // 1001
					websocket.CloseNoStatusReceived, // 1005
					websocket.CloseAbnormalClosure,  // 1006
				) {
					rl.Log.Printf("unexpected close error from %s: %v\n", r.Header.Get("X-Forwarded-For"), err)
				}
				break
			}

			if typ == websocket.PingMessage {
				ws.WriteMessage(websocket.PongMessage, nil)
				continue
			}

			go func(message []byte) {
				ctx = context.Background()

				var request []json.RawMessage
				if err := json.Unmarshal(message, &request); err != nil {
					// stop silently
					return
				}

				if len(request) < 2 {
					ws.WriteJSON(nostr.NoticeEnvelope("request has less than 2 parameters"))
					return
				}

				var typ string
				json.Unmarshal(request[0], &typ)

				switch typ {
				case "EVENT":
					// it's a new event
					var evt nostr.Event
					if err := json.Unmarshal(request[1], &evt); err != nil {
						ws.WriteJSON(nostr.NoticeEnvelope("failed to decode event: " + err.Error()))
						return
					}

					// check serialization
					serialized := evt.Serialize()

					// assign ID
					hash := sha256.Sum256(serialized)
					evt.ID = hex.EncodeToString(hash[:])

					// check signature (requires the ID to be set)
					if ok, err := evt.CheckSignature(); err != nil {
						ws.WriteJSON(nostr.OKEnvelope{EventID: evt.ID, OK: false, Reason: "error: failed to verify signature"})
						return
					} else if !ok {
						ws.WriteJSON(nostr.OKEnvelope{EventID: evt.ID, OK: false, Reason: "invalid: signature is invalid"})
						return
					}

					var ok bool
					if evt.Kind == 5 {
						err = rl.handleDeleteRequest(ctx, &evt)
					} else {
						err = rl.AddEvent(ctx, &evt)
					}

					var reason string
					if err == nil {
						ok = true
					} else {
						reason = err.Error()
					}
					ws.WriteJSON(nostr.OKEnvelope{EventID: evt.ID, OK: ok, Reason: reason})
				case "COUNT":
					if rl.CountEvents == nil {
						ws.WriteJSON(nostr.NoticeEnvelope("this relay does not support NIP-45"))
						return
					}

					var id string
					json.Unmarshal(request[1], &id)
					if id == "" {
						ws.WriteJSON(nostr.NoticeEnvelope("COUNT has no <id>"))
						return
					}

					var total int64
					filters := make(nostr.Filters, len(request)-2)
					for i, filterReq := range request[2:] {
						if err := json.Unmarshal(filterReq, &filters[i]); err != nil {
							ws.WriteJSON(nostr.NoticeEnvelope("failed to decode filter"))
							continue
						}
						total += rl.handleCountRequest(ctx, ws, filters[i])
					}

					ws.WriteJSON([]interface{}{"COUNT", id, map[string]int64{"count": total}})
				case "REQ":
					var id string
					json.Unmarshal(request[1], &id)
					if id == "" {
						ws.WriteJSON(nostr.NoticeEnvelope("REQ has no <id>"))
						return
					}

					filters := make(nostr.Filters, len(request)-2)
					eose := sync.WaitGroup{}
					eose.Add(len(request[2:]))

					for i, filterReq := range request[2:] {
						if err := json.Unmarshal(filterReq, &filters[i]); err != nil {
							ws.WriteJSON(nostr.NoticeEnvelope("failed to decode filter"))
							eose.Done()
							continue
						}

						go rl.handleRequest(ctx, id, &eose, ws, filters[i])
					}

					go func() {
						eose.Wait()
						ws.WriteJSON(nostr.EOSEEnvelope(id))
					}()

					setListener(id, ws, filters)
				case "CLOSE":
					var id string
					json.Unmarshal(request[1], &id)
					if id == "" {
						ws.WriteJSON(nostr.NoticeEnvelope("CLOSE has no <id>"))
						return
					}

					removeListenerId(ws, id)
				case "AUTH":
					if rl.ServiceURL != "" {
						var evt nostr.Event
						if err := json.Unmarshal(request[1], &evt); err != nil {
							ws.WriteJSON(nostr.NoticeEnvelope("failed to decode auth event: " + err.Error()))
							return
						}
						if pubkey, ok := nip42.ValidateAuthEvent(&evt, ws.Challenge, rl.ServiceURL); ok {
							ws.Authed = pubkey
							close(ws.WaitingForAuth)
							ctx = context.WithValue(ctx, AUTH_CONTEXT_KEY, pubkey)
							ws.WriteJSON(nostr.OKEnvelope{EventID: evt.ID, OK: true})
						} else {
							ws.WriteJSON(nostr.OKEnvelope{EventID: evt.ID, OK: false, Reason: "error: failed to authenticate"})
						}
					}
				}
			}(message)
		}
	}()

	// writer
	go func() {
		defer func() {
			ticker.Stop()
			conn.Close()
		}()

		for {
			select {
			case <-ticker.C:
				err := ws.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					if !strings.HasSuffix(err.Error(), "use of closed network connection") {
						rl.Log.Printf("error writing ping: %v; closing websocket\n", err)
					}
					return
				}
			}
		}
	}()
}

func (rl *Relay) HandleNIP11(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/nostr+json")

	supportedNIPs := []int{}
	if rl.ServiceURL != "" {
		supportedNIPs = append(supportedNIPs, 42)
	}
	if rl.CountEvents != nil {
		supportedNIPs = append(supportedNIPs, 45)
	}

	info := nip11.RelayInformationDocument{
		Name:          rl.Name,
		Description:   rl.Description,
		PubKey:        rl.PubKey,
		Contact:       rl.Contact,
		Icon:          rl.IconURL,
		SupportedNIPs: supportedNIPs,
		Software:      "https://github.com/fiatjaf/khatru",
		Version:       "n/a",
	}

	for _, edit := range rl.EditInformation {
		edit(r.Context(), &info)
	}

	json.NewEncoder(w).Encode(info)
}
