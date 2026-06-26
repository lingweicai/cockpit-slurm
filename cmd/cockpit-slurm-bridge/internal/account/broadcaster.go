package account

import (
	"sync"
)

// Broadcaster fan-outs account events to subscribers without blocking the producer.
type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
	bufferSize  int
}

// NewBroadcaster creates a broadcaster with a default subscriber buffer.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan Event]struct{}),
		bufferSize:  16,
	}
}

// Subscribe registers a new subscriber channel.
func (b *Broadcaster) Subscribe() chan Event {
	if b == nil {
		return nil
	}

	ch := make(chan Event, b.bufferSize)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel and closes it.
func (b *Broadcaster) Unsubscribe(ch chan Event) {
	if b == nil || ch == nil {
		return
	}

	b.mu.Lock()
	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
	b.mu.Unlock()
}

// Broadcast sends an event to all subscribers without blocking.
func (b *Broadcaster) Broadcast(evt Event) {
	if b == nil {
		return
	}

	b.mu.RLock()
	subscribers := make([]chan Event, 0, len(b.subscribers))
	for ch := range b.subscribers {
		subscribers = append(subscribers, ch)
	}
	b.mu.RUnlock()

	for _, ch := range subscribers {
		select {
		case ch <- evt:
		default:
		}
	}
}
