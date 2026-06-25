package account

import (
	"testing"
	"time"
)

func TestBroadcasterBroadcastsToAllSubscribers(t *testing.T) {
	b := NewBroadcaster()
	a := b.Subscribe()
	c := b.Subscribe()
	defer b.Unsubscribe(a)
	defer b.Unsubscribe(c)

	evt := Event{Type: "account.updated"}
	b.Broadcast(evt)

	select {
	case got := <-a:
		if got.Type != evt.Type {
			t.Fatalf("subscriber a type = %q, want %q", got.Type, evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for subscriber a")
	}

	select {
	case got := <-c:
		if got.Type != evt.Type {
			t.Fatalf("subscriber c type = %q, want %q", got.Type, evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for subscriber c")
	}
}
