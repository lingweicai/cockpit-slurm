package event

import (
	"sync"
	"sync/atomic"
)

// Action identifies the lifecycle change represented by an event.
type Action string

const (
	ActionAdded   Action = "added"
	ActionUpdated Action = "updated"
	ActionRemoved Action = "removed"
)

// Event is a committed resource change. Subscription-specific sequencing is
// added by the subscription manager when the event is delivered.
type Event struct {
	Resource   string
	Action     Action
	Generation int64
	Node       any
}

type Subscription struct {
	ID     uint64
	Events <-chan Event

	bus  *Bus
	once sync.Once
}

// Close removes the subscription and closes its event stream.
func (s *Subscription) Close() {
	if s == nil || s.bus == nil {
		return
	}
	s.once.Do(func() { s.bus.remove(s.ID) })
}

type subscriber struct {
	channel chan Event
}

// Bus publishes committed resource events to independent subscribers.
type Bus struct {
	mu         sync.Mutex
	subscribers map[uint64]subscriber
	nextID     atomic.Uint64
	bufferSize int
}

func NewBus(bufferSize int) *Bus {
	if bufferSize < 1 {
		bufferSize = 1
	}
	return &Bus{
		subscribers: make(map[uint64]subscriber),
		bufferSize:  bufferSize,
	}
}

func (b *Bus) Subscribe() *Subscription {
	id := b.nextID.Add(1)
	channel := make(chan Event, b.bufferSize)
	b.mu.Lock()
	b.subscribers[id] = subscriber{channel: channel}
	b.mu.Unlock()
	return &Subscription{ID: id, Events: channel, bus: b}
}

// Publish delivers an event to every current subscriber. A subscriber whose
// bounded queue is full is removed so it cannot block other subscribers.
func (b *Bus) Publish(value Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, sub := range b.subscribers {
		select {
		case sub.channel <- value:
		default:
			delete(b.subscribers, id)
			close(sub.channel)
		}
	}
}

func (b *Bus) remove(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sub, ok := b.subscribers[id]; ok {
		delete(b.subscribers, id)
		close(sub.channel)
	}
}