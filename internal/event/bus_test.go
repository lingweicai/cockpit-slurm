package event

import "testing"

func TestBusPublishesToMultipleSubscribers(t *testing.T) {
	bus := NewBus(2)
	first := bus.Subscribe()
	second := bus.Subscribe()
	want := Event{Resource: "nodes", Action: ActionUpdated, Generation: 4}

	bus.Publish(want)
	if got := <-first.Events; got != want {
		t.Fatalf("first event = %#v, want %#v", got, want)
	}
	if got := <-second.Events; got != want {
		t.Fatalf("second event = %#v, want %#v", got, want)
	}
}

func TestBusCloseStopsSubscriber(t *testing.T) {
	bus := NewBus(1)
	subscription := bus.Subscribe()
	subscription.Close()
	if _, open := <-subscription.Events; open {
		t.Fatal("subscription channel remained open")
	}
}

func TestBusRemovesOnlySlowSubscriber(t *testing.T) {
	bus := NewBus(1)
	slow := bus.Subscribe()
	active := bus.Subscribe()

	bus.Publish(Event{Resource: "nodes", Generation: 1})
	if got := (<-active.Events).Generation; got != 1 {
		t.Fatalf("active first generation = %d, want 1", got)
	}
	bus.Publish(Event{Resource: "nodes", Generation: 2})

	if _, open := <-slow.Events; !open {
		t.Fatal("slow subscriber closed before buffered event was delivered")
	}
	if _, open := <-slow.Events; open {
		t.Fatal("slow subscriber should have been removed after queue overflow")
	}
	if got := (<-active.Events).Generation; got != 2 {
		t.Fatalf("active second generation = %d, want 2", got)
	}
}