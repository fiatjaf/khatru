package khatru

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
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

		{
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
		DeletedLoop:
			for {
				select {
				case <-sub.Events:
					gotEvent = true
				case <-sub.EndOfStoredEvents:
					if gotEvent {
						t.Error("should not have received deleted event")
					}
					break DeletedLoop
				case <-ctx.Done():
					t.Fatal("timeout waiting for EOSE")
				}
			}
		}

		{
			// Try to query the deletion itself
			sub, err := client2.Subscribe(ctx, []nostr.Filter{{
				Kinds: []int{5},
			}})
			if err != nil {
				t.Fatalf("failed to subscribe: %v", err)
			}
			defer sub.Unsub()

			// Should get EOSE without receiving the deleted event
			gotEvent := false
		DeletionLoop:
			for {
				select {
				case <-sub.Events:
					gotEvent = true
				case <-sub.EndOfStoredEvents:
					if !gotEvent {
						t.Error("should have received deletion event")
					}
					break DeletionLoop
				case <-ctx.Done():
					t.Fatal("timeout waiting for EOSE")
				}
			}
		}
	})

	t.Run("multi-target deletion", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		expectDeleted := func(eventID string) {
			t.Helper()
			sub, err := client2.Subscribe(ctx, []nostr.Filter{{
				IDs: []string{eventID},
			}})
			if err != nil {
				t.Fatalf("failed to subscribe: %v", err)
			}
			defer sub.Unsub()

			gotEvent := false
			for {
				select {
				case <-sub.Events:
					gotEvent = true
				case <-sub.EndOfStoredEvents:
					if gotEvent {
						t.Errorf("should not have received deleted event %s", eventID)
					}
					return
				case <-ctx.Done():
					t.Fatalf("timeout waiting for EOSE for %s", eventID)
				}
			}
		}

		evtA := createEvent(sk1, 1, "delete me A", nil)
		evtB := createEvent(sk1, 1, "delete me B", nil)
		if err := client1.Publish(ctx, evtA); err != nil {
			t.Fatalf("failed to publish event A: %v", err)
		}
		if err := client1.Publish(ctx, evtB); err != nil {
			t.Fatalf("failed to publish event B: %v", err)
		}

		delEvent := createEvent(sk1, 5, "multi delete", nostr.Tags{{"e", evtA.ID}, {"e", evtB.ID}})
		if err := client1.Publish(ctx, delEvent); err != nil {
			t.Fatalf("failed to publish deletion event: %v", err)
		}

		expectDeleted(evtA.ID)
		expectDeleted(evtB.ID)
	})

	t.Run("missing-first-tag deletion", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		evt := createEvent(sk1, 1, "delete me after missing tag", nil)
		if err := client1.Publish(ctx, evt); err != nil {
			t.Fatalf("failed to publish event: %v", err)
		}

		delEvent := createEvent(sk1, 5, "missing first tag", nostr.Tags{{"e", "missing-id"}, {"e", evt.ID}})
		if err := client1.Publish(ctx, delEvent); err != nil {
			t.Fatalf("failed to publish deletion event: %v", err)
		}

		sub, err := client2.Subscribe(ctx, []nostr.Filter{{
			IDs: []string{evt.ID},
		}})
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}
		defer sub.Unsub()

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

	t.Run("address-based deletion", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		identifier := "addr-1"
		evt := createEvent(sk1, 30001, "addressable", nostr.Tags{{"d", identifier}})
		if err := client1.Publish(ctx, evt); err != nil {
			t.Fatalf("failed to publish addressable event: %v", err)
		}

		address := fmt.Sprintf("%d:%s:%s", evt.Kind, evt.PubKey, identifier)
		delEvent := createEvent(sk1, 5, "delete address", nostr.Tags{{"a", address}})
		if err := client1.Publish(ctx, delEvent); err != nil {
			t.Fatalf("failed to publish deletion event: %v", err)
		}

		sub, err := client2.Subscribe(ctx, []nostr.Filter{{
			IDs: []string{evt.ID},
		}})
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}
		defer sub.Unsub()

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

	t.Run("unauthorized multi-delete", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		expectPresent := func(eventID string) {
			t.Helper()
			sub, err := client2.Subscribe(ctx, []nostr.Filter{{
				IDs: []string{eventID},
			}})
			if err != nil {
				t.Fatalf("failed to subscribe: %v", err)
			}
			defer sub.Unsub()

			select {
			case env := <-sub.Events:
				if env.ID != eventID {
					t.Errorf("got wrong event: %v", env.ID)
				}
			case <-sub.EndOfStoredEvents:
				t.Errorf("expected event %s to exist", eventID)
			case <-ctx.Done():
				t.Fatalf("timeout waiting for event %s", eventID)
			}
		}

		evtOwned := createEvent(sk1, 1, "owned by sk1", nil)
		evtOther := createEvent(sk2, 1, "owned by sk2", nil)
		if err := client1.Publish(ctx, evtOwned); err != nil {
			t.Fatalf("failed to publish owned event: %v", err)
		}
		if err := client2.Publish(ctx, evtOther); err != nil {
			t.Fatalf("failed to publish other event: %v", err)
		}

		delEvent := createEvent(sk1, 5, "invalid multi delete", nostr.Tags{{"e", evtOwned.ID}, {"e", evtOther.ID}})
		err := client1.Publish(ctx, delEvent)
		if err == nil {
			t.Fatal("should have failed to publish deletion event")
		}
		if !strings.Contains(err.Error(), "blocked:") {
			t.Fatalf("expected blocked error, got: %v", err)
		}

		expectPresent(evtOwned.ID)
		expectPresent(evtOther.ID)
	})

	// test 4: replaceable events
	t.Run("replaceable events", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// create initial kind:0 event
		evt1 := createEvent(sk1, 0, `{"name":"initial"}`, nil)
		evt1.CreatedAt = 1000 // Set specific timestamp for testing
		evt1.Sign(sk1)
		err = client1.Publish(ctx, evt1)
		if err != nil {
			t.Fatalf("failed to publish initial event: %v", err)
		}

		// create newer event that should replace the first
		evt2 := createEvent(sk1, 0, `{"name":"newer"}`, nil)
		evt2.CreatedAt = 2000 // Newer timestamp
		evt2.Sign(sk1)
		err = client1.Publish(ctx, evt2)
		if err != nil {
			t.Fatalf("failed to publish newer event: %v", err)
		}

		// create older event that should not replace the current one
		evt3 := createEvent(sk1, 0, `{"name":"older"}`, nil)
		evt3.CreatedAt = 1500 // Older than evt2
		evt3.Sign(sk1)
		err = client1.Publish(ctx, evt3)
		if err != nil {
			t.Fatalf("failed to publish older event: %v", err)
		}

		// query to verify only the newest event exists
		sub, err := client2.Subscribe(ctx, []nostr.Filter{{
			Authors: []string{pk1},
			Kinds:   []int{0},
		}})
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}
		defer sub.Unsub()

		// should only get one event back (the newest one)
		var receivedEvents []*nostr.Event
		for {
			select {
			case env := <-sub.Events:
				receivedEvents = append(receivedEvents, env)
			case <-sub.EndOfStoredEvents:
				if len(receivedEvents) != 1 {
					t.Errorf("expected exactly 1 event, got %d", len(receivedEvents))
				}
				if len(receivedEvents) > 0 && receivedEvents[0].Content != `{"name":"newer"}` {
					t.Errorf("expected newest event content, got %s", receivedEvents[0].Content)
				}
				return
			case <-ctx.Done():
				t.Fatal("timeout waiting for events")
			}
		}
	})

	// test 5: event expiration
	t.Run("event expiration", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// create a new relay with shorter expiration check interval
		relay := NewRelay()
		relay.expirationManager.interval = 3 * time.Second // check every 3 seconds
		store := slicestore.SliceStore{}
		store.Init()
		relay.StoreEvent = append(relay.StoreEvent, store.SaveEvent)
		relay.QueryEvents = append(relay.QueryEvents, store.QueryEvents)
		relay.DeleteEvent = append(relay.DeleteEvent, store.DeleteEvent)

		// start test server
		server := httptest.NewServer(relay)
		defer server.Close()

		// connect test client
		url := "ws" + server.URL[4:]
		client, err := nostr.RelayConnect(context.Background(), url)
		if err != nil {
			t.Fatalf("failed to connect client: %v", err)
		}
		defer client.Close()

		// create event that expires in 2 seconds
		expiration := strconv.FormatInt(int64(nostr.Now()+2), 10)
		evt := createEvent(sk1, 1, "i will expire soon", nostr.Tags{{"expiration", expiration}})
		err = client.Publish(ctx, evt)
		if err != nil {
			t.Fatalf("failed to publish event: %v", err)
		}

		// verify event exists initially
		sub, err := client.Subscribe(ctx, []nostr.Filter{{
			IDs: []string{evt.ID},
		}})
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}

		// should get the event
		select {
		case env := <-sub.Events:
			if env.ID != evt.ID {
				t.Error("got wrong event")
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for event")
		}
		sub.Unsub()

		// wait for expiration check (>3 seconds)
		time.Sleep(4 * time.Second)

		// verify event no longer exists
		sub, err = client.Subscribe(ctx, []nostr.Filter{{
			IDs: []string{evt.ID},
		}})
		if err != nil {
			t.Fatalf("failed to subscribe: %v", err)
		}
		defer sub.Unsub()

		// should get EOSE without receiving the expired event
		gotEvent := false
		for {
			select {
			case <-sub.Events:
				gotEvent = true
			case <-sub.EndOfStoredEvents:
				if gotEvent {
					t.Error("should not have received expired event")
				}
				return
			case <-ctx.Done():
				t.Fatal("timeout waiting for EOSE")
			}
		}
	})

	// test 6: unauthorized deletion
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
