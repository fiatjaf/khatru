package khatru

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip86"
)

type RelayManagementAPI struct {
	RejectAPICall []func(ctx context.Context, mp nip86.MethodParams) (reject bool, msg string)

	BanPubKey                   func(ctx context.Context, pubkey string, reason string) error
	ListBannedPubKeys           func(ctx context.Context) ([]nip86.PubKeyReason, error)
	AllowPubKey                 func(ctx context.Context, pubkey string, reason string) error
	ListAllowedPubKeys          func(ctx context.Context) ([]nip86.PubKeyReason, error)
	ListEventsNeedingModeration func(ctx context.Context) ([]nip86.IDReason, error)
	AllowEvent                  func(ctx context.Context, id string, reason string) error
	BanEvent                    func(ctx context.Context, id string, reason string) error
	ListBannedEvents            func(ctx context.Context) ([]nip86.IDReason, error)
	ChangeRelayName             func(ctx context.Context, name string) error
	ChangeRelayDescription      func(ctx context.Context, desc string) error
	ChangeRelayIcon             func(ctx context.Context, icon string) error
	AllowKind                   func(ctx context.Context, kind int) error
	DisallowKind                func(ctx context.Context, kind int) error
	ListAllowedKinds            func(ctx context.Context) ([]int, error)
	BlockIP                     func(ctx context.Context, ip net.IP, reason string) error
	UnblockIP                   func(ctx context.Context, ip net.IP, reason string) error
	ListBlockedIPs              func(ctx context.Context) ([]nip86.IPReason, error)
}

func (rl *Relay) HandleNIP86(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/nostr+json+rpc")

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "empty request", 400)
		return
	}
	payloadHash := sha256.Sum256(nil)

	auth := r.Header.Get("Authorization")
	spl := strings.Split(auth, "Nostr ")
	if len(spl) != 2 {
		http.Error(w, "missing auth", 401)
		return
	}

	var evt nostr.Event
	if evtj, err := base64.StdEncoding.DecodeString(spl[1]); err != nil {
		http.Error(w, "invalid base64 auth", 401)
		return
	} else if err := json.Unmarshal(evtj, &evt); err != nil {
		http.Error(w, "invalid auth event json", 401)
		return
	} else if ok, _ := evt.CheckSignature(); !ok {
		http.Error(w, "invalid auth event", 401)
		return
	} else if pht := evt.Tags.GetFirst([]string{"payload", hex.EncodeToString(payloadHash[:])}); pht == nil {
		http.Error(w, "invalid auth event payload hash", 401)
		return
	} else if evt.CreatedAt < nostr.Now()-30 {
		http.Error(w, "auth event is too old", 401)
		return
	}

	var req nip86.Request
	if err := json.Unmarshal(payload, &req); err != nil {
		http.Error(w, "invalid json body", 400)
		return
	}

	mp, err := nip86.DecodeRequest(req)
	if err != nil {
		http.Error(w, "invalid params: "+err.Error(), 400)
		return
	}

	var resp nip86.Response

	ctx := context.WithValue(r.Context(), nip86HeaderAuthKey, evt.PubKey)
	for _, rac := range rl.ManagementAPI.RejectAPICall {
		if reject, msg := rac(ctx, mp); reject {
			resp.Error = msg
			goto respond
		}
	}

	if _, ok := mp.(nip86.SupportedMethods); ok {
		mat := reflect.TypeOf(rl.ManagementAPI)
		mav := reflect.ValueOf(rl.ManagementAPI)

		methods := make([]string, 0, mat.NumField())
		for i := 0; i < mat.NumField(); i++ {
			field := mat.Field(i)

			// danger: this assumes the struct fields are appropriately named
			methodName := strings.ToLower(field.Name)

			// assign this only if the function was defined
			if mav.Field(i).Interface() != nil {
				methods[i] = methodName
			}
		}
		resp.Result = methods
	} else {
		switch thing := mp.(type) {
		case nip86.BanPubKey:
			if rl.ManagementAPI.BanPubKey == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.BanPubKey(ctx, thing.PubKey, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ListBannedPubKeys:
			if rl.ManagementAPI.ListBannedPubKeys == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListBannedPubKeys(ctx); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		case nip86.AllowPubKey:
			if rl.ManagementAPI.AllowPubKey == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.AllowPubKey(ctx, thing.PubKey, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ListAllowedPubKeys:
			if rl.ManagementAPI.ListAllowedPubKeys == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListAllowedPubKeys(ctx); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		case nip86.BanEvent:
			if rl.ManagementAPI.BanEvent == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.BanEvent(ctx, thing.ID, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.AllowEvent:
			if rl.ManagementAPI.AllowEvent == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.AllowEvent(ctx, thing.ID, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ListEventsNeedingModeration:
			if rl.ManagementAPI.ListEventsNeedingModeration == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListEventsNeedingModeration(ctx); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		case nip86.ListBannedEvents:
			if rl.ManagementAPI.ListBannedEvents == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListEventsNeedingModeration(ctx); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		case nip86.ChangeRelayName:
			if rl.ManagementAPI.ChangeRelayName == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.ChangeRelayName(ctx, thing.Name); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ChangeRelayDescription:
			if rl.ManagementAPI.ChangeRelayDescription == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.ChangeRelayDescription(ctx, thing.Description); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ChangeRelayIcon:
			if rl.ManagementAPI.ChangeRelayIcon == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.ChangeRelayIcon(ctx, thing.IconURL); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.AllowKind:
			if rl.ManagementAPI.AllowKind == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.AllowKind(ctx, thing.Kind); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.DisallowKind:
			if rl.ManagementAPI.DisallowKind == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.DisallowKind(ctx, thing.Kind); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ListAllowedKinds:
			if rl.ManagementAPI.ListAllowedKinds == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListAllowedKinds(ctx); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		case nip86.BlockIP:
			if rl.ManagementAPI.BlockIP == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.BlockIP(ctx, thing.IP, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.UnblockIP:
			if rl.ManagementAPI.UnblockIP == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.UnblockIP(ctx, thing.IP, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ListBlockedIPs:
			if rl.ManagementAPI.ListBlockedIPs == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListBlockedIPs(ctx); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		default:
			resp.Error = fmt.Sprintf("method '%s' not known", mp.MethodName())
		}
	}

respond:
	json.NewEncoder(w).Encode(resp)
}
