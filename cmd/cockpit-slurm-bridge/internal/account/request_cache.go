package account

import (
	"sync"
	"time"
)

type requestEntry struct {
	done      chan struct{}
	result    RequestResult
	expiresAt time.Time
}

// RequestCache stores completed request outcomes and tracks in-flight requests.
type RequestCache struct {
	mu        sync.Mutex
	ttl       time.Duration
	completed map[string]requestEntry
	inFlight  map[string]*requestEntry
}

// NewRequestCache creates a request cache with the provided TTL.
func NewRequestCache(ttl time.Duration) *RequestCache {
	return &RequestCache{
		ttl:       ttl,
		completed: make(map[string]requestEntry),
		inFlight:  make(map[string]*requestEntry),
	}
}

// Begin reserves a request ID or returns the cached result / in-flight signal.
func (c *RequestCache) Begin(requestID string) (RequestResult, bool, <-chan struct{}) {
	if c == nil || requestID == "" {
		return RequestResult{}, false, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanupLocked(time.Now().UTC())

	if entry, ok := c.completed[requestID]; ok {
		return entry.result, true, nil
	}
	if entry, ok := c.inFlight[requestID]; ok {
		return RequestResult{}, false, entry.done
	}

	entry := &requestEntry{done: make(chan struct{}), expiresAt: time.Time{}}
	c.inFlight[requestID] = entry
	return RequestResult{}, false, nil
}

// Complete records the final result and releases waiters.
func (c *RequestCache) Complete(requestID string, result RequestResult) {
	if c == nil || requestID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.inFlight[requestID]
	if !ok {
		return
	}

	result.RequestID = requestID
	result.Timestamp = time.Now().UTC()
	entry.result = result
	entry.expiresAt = result.Timestamp.Add(c.ttl)
	c.completed[requestID] = *entry
	delete(c.inFlight, requestID)
	close(entry.done)
}

// Get returns a cached completed request if it is still valid.
func (c *RequestCache) Get(requestID string) (RequestResult, bool) {
	if c == nil || requestID == "" {
		return RequestResult{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanupLocked(time.Now().UTC())

	entry, ok := c.completed[requestID]
	if !ok {
		return RequestResult{}, false
	}

	return entry.result, true
}

func (c *RequestCache) cleanupLocked(now time.Time) {
	if c.ttl <= 0 {
		return
	}

	for id, entry := range c.completed {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			delete(c.completed, id)
		}
	}
}
