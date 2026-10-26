package resource

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type nodeAdapterFunc func(context.Context) ([]Node, error)

func (f nodeAdapterFunc) ListNodes(ctx context.Context) ([]Node, error) {
	return f(ctx)
}

type nodePublisherFunc func(NodeChangeBatch)

func (f nodePublisherFunc) Publish(batch NodeChangeBatch) {
	f(batch)
}

func TestNodeSynchronizerRefreshPublishesOnlyChangedSnapshots(t *testing.T) {
	cache := NewNodeCache()
	node := NewNode("node001", 0)
	node.Status.State = "IDLE"
	calls := 0
	adapter := nodeAdapterFunc(func(context.Context) ([]Node, error) {
		calls++
		return []Node{node}, nil
	})
	published := make(chan NodeChangeBatch, 2)
	synchronizer, err := NewNodeSynchronizer(adapter, cache, nodePublisherFunc(func(batch NodeChangeBatch) {
		published <- batch
	}), time.Second)
	if err != nil {
		t.Fatalf("NewNodeSynchronizer returned error: %v", err)
	}

	if err := synchronizer.Refresh(context.Background()); err != nil {
		t.Fatalf("first Refresh returned error: %v", err)
	}
	if err := synchronizer.Refresh(context.Background()); err != nil {
		t.Fatalf("second Refresh returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("adapter calls = %d, want 2", calls)
	}
	if got := cache.Generation(); got != 1 {
		t.Fatalf("generation = %d, want 1", got)
	}
	select {
	case batch := <-published:
		if batch.Generation != 1 || len(batch.Changes) != 1 || batch.Changes[0].Kind != NodeAdded {
			t.Fatalf("published batch = %#v, want one added Node at generation 1", batch)
		}
	case <-time.After(time.Second):
		t.Fatal("expected changed snapshot to be published")
	}
	select {
	case batch := <-published:
		t.Fatalf("unchanged snapshot published an extra batch: %#v", batch)
	default:
	}
}

func TestNodeSynchronizerFailedOrInvalidRefreshPreservesCache(t *testing.T) {
	cache := NewNodeCache()
	published := 0
	current := []Node{NewNode("node001", 0)}
	var adapterErr error
	adapter := nodeAdapterFunc(func(context.Context) ([]Node, error) {
		return current, adapterErr
	})
	synchronizer, err := NewNodeSynchronizer(adapter, cache, nodePublisherFunc(func(NodeChangeBatch) {
		published++
	}), time.Second)
	if err != nil {
		t.Fatalf("NewNodeSynchronizer returned error: %v", err)
	}
	if err := synchronizer.Refresh(context.Background()); err != nil {
		t.Fatalf("initial Refresh returned error: %v", err)
	}

	adapterErr = errors.New("Slurm unavailable")
	if err := synchronizer.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh succeeded despite adapter failure")
	}
	adapterErr = nil

	current = []Node{NewNode("", 0)}
	if err := synchronizer.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh succeeded with a Node missing its identity")
	}
	current = []Node{NewNode("node001", 0), NewNode("node001", 0)}
	if err := synchronizer.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh succeeded with duplicate identities")
	}

	if got := cache.Generation(); got != 1 {
		t.Fatalf("generation = %d, want 1 after failed refreshes", got)
	}
	if _, ok := cache.Get("node001"); !ok {
		t.Fatal("failed refresh removed the cached node")
	}
	if published != 1 {
		t.Fatalf("published batches = %d, want only the initial batch", published)
	}
}

func TestNodeSynchronizerRunRefreshesUntilContextCancellation(t *testing.T) {
	var calls atomic.Int32
	adapter := nodeAdapterFunc(func(context.Context) ([]Node, error) {
		calls.Add(1)
		return []Node{NewNode("node001", 0)}, nil
	})
	synchronizer, err := NewNodeSynchronizer(adapter, NewNodeCache(), nil, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("NewNodeSynchronizer returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- synchronizer.Run(ctx)
	}()

	deadline := time.After(time.Second)
	for calls.Load() < 3 {
		select {
		case <-deadline:
			cancel()
			t.Fatalf("periodic refresh calls = %d, want at least 3", calls.Load())
		case <-time.After(time.Millisecond):
		}
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error after cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func TestNewNodeSynchronizerRequiresValidDependencies(t *testing.T) {
	adapter := nodeAdapterFunc(func(context.Context) ([]Node, error) { return nil, nil })
	cache := NewNodeCache()
	for _, test := range []struct {
		name     string
		adapter  NodeAdapter
		cache    *NodeCache
		interval time.Duration
	}{
		{name: "missing adapter", cache: cache, interval: time.Second},
		{name: "missing cache", adapter: adapter, interval: time.Second},
		{name: "zero interval", adapter: adapter, cache: cache},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewNodeSynchronizer(test.adapter, test.cache, nil, test.interval); err == nil {
				t.Fatal("NewNodeSynchronizer succeeded with invalid configuration")
			}
		})
	}
}
