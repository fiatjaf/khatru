package khatru

import (
	"context"
	"testing"
)

func TestMultiplePubkeyAuthenticationDirect(t *testing.T) {
	// test the AUTH handler logic directly by simulating authenticated pubkeys
	ws := &WebSocket{}

	// simulate first auth
	ws.authLock.Lock()
	if ws.AuthedPublicKeys == nil {
		ws.AuthedPublicKeys = make(map[string]struct{})
	}
	ws.AuthedPublicKeys["pubkey1"] = struct{}{}
	ws.authLock.Unlock()

	// verify first auth
	if _, ok := ws.AuthedPublicKeys["pubkey1"]; !ok {
		t.Error("pubkey1 should be authenticated after first auth")
	}

	// simulate second auth (should add, not overwrite)
	ws.authLock.Lock()
	if ws.AuthedPublicKeys == nil {
		ws.AuthedPublicKeys = make(map[string]struct{})
	}
	ws.AuthedPublicKeys["pubkey2"] = struct{}{}
	ws.authLock.Unlock()

	// verify both pubkeys are now authenticated
	if _, ok := ws.AuthedPublicKeys["pubkey1"]; !ok {
		t.Error("pubkey1 should still be authenticated after second auth")
	}
	if _, ok := ws.AuthedPublicKeys["pubkey2"]; !ok {
		t.Error("pubkey2 should be authenticated after second auth")
	}

	if len(ws.AuthedPublicKeys) != 2 {
		t.Errorf("expected 2 authenticated pubkeys, got %d", len(ws.AuthedPublicKeys))
	}

	// test IsAuthed through context
	ctx := context.WithValue(context.Background(), wsKey, ws)

	if !IsAuthed(ctx, "pubkey1") {
		t.Error("IsAuthed should return true for pubkey1")
	}
	if !IsAuthed(ctx, "pubkey2") {
		t.Error("IsAuthed should return true for pubkey2")
	}
	if IsAuthed(ctx, "pubkey3") {
		t.Error("IsAuthed should return false for non-authenticated pubkey3")
	}
}

func TestIsAuthedFunction(t *testing.T) {
	// create a mock websocket with two authenticated pubkeys
	ws := &WebSocket{
		AuthedPublicKeys: map[string]struct{}{
			"pubkey1": {},
			"pubkey2": {},
		},
	}

	// create context with the websocket
	ctx := context.WithValue(context.Background(), wsKey, ws)

	// test IsAuthed for authenticated pubkeys
	if !IsAuthed(ctx, "pubkey1") {
		t.Error("pubkey1 should be authenticated")
	}
	if !IsAuthed(ctx, "pubkey2") {
		t.Error("pubkey2 should be authenticated")
	}

	// test IsAuthed for non-authenticated pubkey
	if IsAuthed(ctx, "pubkey3") {
		t.Error("pubkey3 should NOT be authenticated")
	}
}

func TestGetAuthedFunction(t *testing.T) {
	// create a mock websocket with authenticated pubkeys
	ws := &WebSocket{
		AuthedPublicKeys: map[string]struct{}{
			"pubkey1": {},
			"pubkey2": {},
		},
	}

	// create context with the websocket
	ctx := context.WithValue(context.Background(), wsKey, ws)

	// test GetAuthed returns one of the authenticated pubkeys
	authed := GetAuthed(ctx)
	if authed != "pubkey1" && authed != "pubkey2" {
		t.Errorf("GetAuthed should return one of the authenticated pubkeys, got: %s", authed)
	}
}

func TestGetAuthedKeysFunction(t *testing.T) {
	// create a mock websocket with authenticated pubkeys
	ws := &WebSocket{
		AuthedPublicKeys: map[string]struct{}{
			"pubkey1": {},
			"pubkey2": {},
		},
	}

	// create context with the websocket
	ctx := context.WithValue(context.Background(), wsKey, ws)

	// test GetAuthedKeys returns all authenticated pubkeys
	keys := GetAuthedKeys(ctx)
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}

	// verify both keys are present
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}
	if !keyMap["pubkey1"] || !keyMap["pubkey2"] {
		t.Error("GetAuthedKeys should return all authenticated pubkeys")
	}
}

func TestNoAuthenticatedKeys(t *testing.T) {
	// create a mock websocket with no authenticated pubkeys
	ws := &WebSocket{}

	// create context with the websocket
	ctx := context.WithValue(context.Background(), wsKey, ws)

	// test GetAuthed returns empty string when no keys authenticated
	if GetAuthed(ctx) != "" {
		t.Error("GetAuthed should return empty string when no keys authenticated")
	}

	// test IsAuthed returns false for any pubkey
	if IsAuthed(ctx, "anykey") {
		t.Error("IsAuthed should return false when no keys authenticated")
	}

	// test GetAuthedKeys returns nil/empty
	keys := GetAuthedKeys(ctx)
	if keys != nil {
		t.Error("GetAuthedKeys should return nil when no keys authenticated")
	}
}

func TestNip86HeaderAuthWithIsAuthed(t *testing.T) {
	// create context with nip86 header auth
	ctx := context.WithValue(context.Background(), nip86HeaderAuthKey, "headerpubkey")

	// test IsAuthed works with nip86 header auth
	if !IsAuthed(ctx, "headerpubkey") {
		t.Error("IsAuthed should return true for nip86 header auth pubkey")
	}

	if IsAuthed(ctx, "otherpubkey") {
		t.Error("IsAuthed should return false for other pubkeys when using nip86 header auth")
	}

	// test GetAuthed returns the header auth pubkey
	if GetAuthed(ctx) != "headerpubkey" {
		t.Error("GetAuthed should return the nip86 header auth pubkey")
	}

	// test GetAuthedKeys returns the header auth pubkey
	keys := GetAuthedKeys(ctx)
	if len(keys) != 1 || keys[0] != "headerpubkey" {
		t.Error("GetAuthedKeys should return the nip86 header auth pubkey")
	}
}
