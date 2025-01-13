package khatru

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fiatjaf/eventstore/slicestore"
	"github.com/nbd-wtf/go-nostr"
)

func TestBasicRelayFunctionality(t *testing.T) {
	// setup relay with in-memory store
	relay := NewRelay()
	store := slicestore.SliceStore{}
	store.Init()
	relay.StoreEvent = append(relay.StoreEvent, store.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, store.QueryEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, store.DeleteEvent)

	// start test server
	server := httptest.NewServer(relay)
	defer server.Close()

	// create test keys
	sk1 := nostr.GeneratePrivateKey()
	pk1, err := nostr.GetPublicKey(sk1)
	if err != nil {
		t.Fatalf("Failed to get public key 1: %v", err)
	}
	sk2 := nostr.GeneratePrivateKey()
	pk2, err := nostr.GetPublicKey(sk2)
	if err != nil {
		t.Fatalf("Failed to get public key 2: %v", err)
	}

	// helper to create signed events
	createEvent := func(sk string, kind int, content string, tags nostr.Tags) nostr.Event {
		pk, err := nostr.GetPublicKey(sk)
		if err != nil {
			t.Fatalf("Failed to get public key: %v", err)
		}
		evt := nostr.Event{
			PubKey:    pk,
			CreatedAt: nostr.Now(),
			Kind:      kind,
			Tags:      tags,
			Content:   content,
		}
		evt.Sign(sk)
		return evt
	}

	// connect two test clients
	url := "ws" + server.URL[4:]
	client1, err := nostr.RelayConnect(context.Background(), url)
	if err != nil {
		t.Fatalf("failed to connect client1: %v", err)
	}
	defer client1.Close()

	client2, err := nostr.RelayConnect(context.Background(), url)
	if err != nil {
		t.Fatalf("failed to connect client2: %v", err)
	}
	defer client2.Close()

	// test 1: store and query events
	t.Run("store and query events", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		evt1 := createEvent(sk1, 1, "hello world", nil)
		err := client1.Publish(ctx, evt1)
		if err != nil {
			t.Fatalf("failed to publish event: %v", err)
		}

		// Query the event back
		sub, err := client2.Subscribe(ctx, []nostr.Filter{{
			Authors: []string{pk1},
			Kinds:   []int{1},
		}})
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}
		defer sub.Unsub()

		// Wait for event
		select {
		case env := <-sub.Events:
			if env.ID != evt1.ID {
				t.Errorf("got wrong event: %v", env.ID)
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for event")
		}
	})

	// test 2: live event subscription
	t.Run("live event subscription", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Setup subscription first
		sub, err := client1.Subscribe(ctx, []nostr.Filter{{
			Authors: []string{pk2},
			Kinds:   []int{1},
		}})
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}
		defer sub.Unsub()

		// Publish event from client2
		evt2 := createEvent(sk2, 1, "testing live events", nil)
		err = client2.Publish(ctx, evt2)
		if err != nil {
			t.Fatalf("failed to publish event: %v", err)
		}

		// Wait for event on subscription
		select {
		case env := <-sub.Events:
			if env.ID != evt2.ID {
				t.Errorf("got wrong event: %v", env.ID)
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for live event")
		}
	})

	// test 3: event deletion
	t.Run("event deletion", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create an event to be deleted
		evt3 := createEvent(sk1, 1, "delete me", nil)
		err = client1.Publish(ctx, evt3)
		if err != nil {
			t.Fatalf("failed to publish event: %v", err)
		}

		// Create deletion event
		delEvent := createEvent(sk1, 5, "deleting", nostr.Tags{{"e", evt3.ID}})
		err = client1.Publish(ctx, delEvent)
		if err != nil {
			t.Fatalf("failed to publish deletion event: %v", err)
		}

		// Try to query the deleted event
		sub, err := client2.Subscribe(ctx, []nostr.Filter{{
			IDs: []string{evt3.ID},
		}})
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}
		defer sub.Unsub()

		// Should get EOSE without receiving the deleted event
		gotEvent := false
		for {
			select {
			case <-sub.Events:
				gotEvent = true
			case <-sub.EndOfStoredEvents:
				if gotEvent {
					t.Error("should not have received deleted event")
				}
				return
			case <-ctx.Done():
				t.Fatal("timeout waiting for EOSE")
			}
		}
	})

	// Test 4: Unauthorized deletion
	t.Run("unauthorized deletion", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// create an event from client1
		evt4 := createEvent(sk1, 1, "try to delete me", nil)
		err = client1.Publish(ctx, evt4)
		if err != nil {
			t.Fatalf("failed to publish event: %v", err)
		}

		// Try to delete it with client2
		delEvent := createEvent(sk2, 5, "trying to delete", nostr.Tags{{"e", evt4.ID}})
		err = client2.Publish(ctx, delEvent)
		if err == nil {
			t.Fatalf("should have failed to publish deletion event: %v", err)
		}

		// Verify event still exists
		sub, err := client1.Subscribe(ctx, []nostr.Filter{{
			IDs: []string{evt4.ID},
		}})
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}
		defer sub.Unsub()

		select {
		case env := <-sub.Events:
			if env.ID != evt4.ID {
				t.Error("got wrong event")
			}
		case <-ctx.Done():
			t.Fatal("event should still exist")
		}
	})
}
