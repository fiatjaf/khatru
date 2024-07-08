package khatru

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"

	"github.com/nbd-wtf/go-nostr/nip86"
)

type RelayManagementAPI struct {
	BanPubKey                   func(pubkey string, reason string) error
	ListBannedPubKeys           func() ([]nip86.PubKeyReason, error)
	AllowPubKey                 func(pubkey string, reason string) error
	ListAllowedPubKeys          func() ([]nip86.PubKeyReason, error)
	ListEventsNeedingModeration func() ([]nip86.IDReason, error)
	AllowEvent                  func(id string, reason string) error
	BanEvent                    func(id string, reason string) error
	ListBannedEvents            func() ([]nip86.IDReason, error)
	ChangeRelayName             func(name string) error
	ChangeRelayDescription      func(desc string) error
	ChangeRelayIcon             func(icon string) error
	AllowKind                   func(kind int) error
	DisallowKind                func(kind int) error
	ListAllowedKinds            func() ([]int, error)
	BlockIP                     func(ip net.IP, reason string) error
	UnblockIP                   func(ip net.IP, reason string) error
	ListBlockedIPs              func() ([]nip86.IPReason, error)
}

func (rl *Relay) HandleNIP86(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/nostr+json+rpc")

	var req nip86.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", 400)
		return
	}

	mp, err := nip86.DecodeRequest(req)
	if err != nil {
		http.Error(w, "invalid params: "+err.Error(), 400)
		return
	}

	var resp nip86.Response
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
			} else if err := rl.ManagementAPI.BanPubKey(thing.PubKey, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ListBannedPubKeys:
			if rl.ManagementAPI.ListBannedPubKeys == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListBannedPubKeys(); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		case nip86.AllowPubKey:
			if rl.ManagementAPI.AllowPubKey == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.AllowPubKey(thing.PubKey, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ListAllowedPubKeys:
			if rl.ManagementAPI.ListAllowedPubKeys == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListAllowedPubKeys(); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		case nip86.BanEvent:
			if rl.ManagementAPI.BanEvent == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.BanEvent(thing.ID, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.AllowEvent:
			if rl.ManagementAPI.AllowEvent == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.AllowEvent(thing.ID, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ListEventsNeedingModeration:
			if rl.ManagementAPI.ListEventsNeedingModeration == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListEventsNeedingModeration(); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		case nip86.ListBannedEvents:
			if rl.ManagementAPI.ListBannedEvents == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListEventsNeedingModeration(); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		case nip86.ChangeRelayName:
			if rl.ManagementAPI.ChangeRelayName == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.ChangeRelayName(thing.Name); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ChangeRelayDescription:
			if rl.ManagementAPI.ChangeRelayDescription == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.ChangeRelayDescription(thing.Description); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ChangeRelayIcon:
			if rl.ManagementAPI.ChangeRelayIcon == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.ChangeRelayIcon(thing.IconURL); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.AllowKind:
			if rl.ManagementAPI.AllowKind == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.AllowKind(thing.Kind); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.DisallowKind:
			if rl.ManagementAPI.DisallowKind == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.DisallowKind(thing.Kind); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ListAllowedKinds:
			if rl.ManagementAPI.ListAllowedKinds == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListAllowedKinds(); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		case nip86.BlockIP:
			if rl.ManagementAPI.BlockIP == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.BlockIP(thing.IP, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.UnblockIP:
			if rl.ManagementAPI.UnblockIP == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if err := rl.ManagementAPI.UnblockIP(thing.IP, thing.Reason); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = true
			}
		case nip86.ListBlockedIPs:
			if rl.ManagementAPI.ListBlockedIPs == nil {
				resp.Error = fmt.Sprintf("method %s not supported", thing.MethodName())
			} else if result, err := rl.ManagementAPI.ListBlockedIPs(); err != nil {
				resp.Error = err.Error()
			} else {
				resp.Result = result
			}
		default:
			resp.Error = fmt.Sprintf("method '%s' not known", mp.MethodName())
		}
	}

	json.NewEncoder(w).Encode(resp)
}
