package resource

import (
	"context"
	"errors"
	"testing"
	"time"
)

type sequenceNodeAdapter struct {
	results [][]Node
	errors  []error
	index   int
}

func (a *sequenceNodeAdapter) ListNodes(context.Context) ([]Node, error) {
	index := a.index
	a.index++
	if index < len(a.errors) && a.errors[index] != nil {
		return nil, a.errors[index]
	}
	if index >= len(a.results) {
		return nil, errors.New("no result configured")
	}
	return a.results[index], nil
}

func TestNodeSynchronizerRefreshesChangedState(t *testing.T) {
	cache := NewNodeCache()
	adapter := &sequenceNodeAdapter{
		results: [][]Node{
			{NewNode("node001", 0)},
			{func() Node {
				node := NewNode("node001", 0)
				node.Status.State = "DOWN"
				return node
			}()},
		},
	}
	var snapshots []Snapshot
	synchronizer, err := NewNodeSynchronizer(adapter, cache, time.Second, func(snapshot Snapshot) {
		 snapshots = append(snapshots, snapshot)
	})
	if err != nil {
		t.Fatal(err)
	}

	changed, err := synchronizer.Refresh(context.Background())
	if err != nil || !changed {
		t.Fatalf("initial refresh = changed %v, err %v; want changed", changed, err)
	}
	changed, err = synchronizer.Refresh(context.Background())
	if err != nil || !changed {
		t.Fatalf("changed refresh = changed %v, err %v; want changed", changed, err)
	}
	if cache.Generation() != 2 {
		t.Fatalf("generation = %d, want 2", cache.Generation())
	}
	if len(snapshots) != 2 || snapshots[1].Nodes[0].Status.State != "DOWN" {
		t.Fatalf("unexpected change callbacks: %#v", snapshots)
	}
}

func TestNodeSynchronizerUnchangedAndFailedRefreshPreserveCache(t *testing.T) {
	cache := NewNodeCache()
	initial := NewNode("node001", 0)
	adapter := &sequenceNodeAdapter{
		results: [][]Node{{initial}, {initial}},
		errors:  []error{nil, nil, errors.New("Slurm unavailable")},
	}
	synchronizer, err := NewNodeSynchronizer(adapter, cache, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}

	changed, err := synchronizer.Refresh(context.Background())
	if err != nil || !changed {
		t.Fatalf("initial refresh = changed %v, err %v; want changed", changed, err)
	}
	changed, err = synchronizer.Refresh(context.Background())
	if err != nil || changed {
		t.Fatalf("unchanged refresh = changed %v, err %v; want unchanged", changed, err)
	}
	if _, err = synchronizer.Refresh(context.Background()); err == nil {
		t.Fatal("failed refresh returned nil error")
	}
	if cache.Generation() != 1 || cache.Snapshot().Nodes[0].Status.State != "" {
		t.Fatalf("cache changed after failed refresh: generation=%d", cache.Generation())
	}
}