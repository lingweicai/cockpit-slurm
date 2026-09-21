package resource

import "testing"

func TestNodeCacheReplaceSnapshotAndGeneration(t *testing.T) {
	cache := NewNodeCache()

	first := []Node{NewNode("node001", 0)}
	cache.ReplaceSnapshot(first)
	if cache.Generation() != 1 {
		t.Fatalf("generation = %d, want 1", cache.Generation())
	}
	if len(cache.Snapshot().Nodes) != 1 {
		t.Fatalf("snapshot len = %d, want 1", len(cache.Snapshot().Nodes))
	}
	if _, ok := cache.Get("node001"); !ok {
		t.Fatal("expected node001 to be present")
	}

	second := []Node{NewNode("node002", 0)}
	cache.ReplaceSnapshot(second)
	if cache.Generation() != 2 {
		t.Fatalf("generation = %d, want 2", cache.Generation())
	}
	if _, ok := cache.Get("node001"); ok {
		t.Fatal("node001 should have been replaced from cache")
	}
	if _, ok := cache.Get("node002"); !ok {
		t.Fatal("expected node002 to be present")
	}
}

func TestNodeCachePreservesPreviousSnapshotOnReplaceFailure(t *testing.T) {
	cache := NewNodeCache()
	cache.ReplaceSnapshot([]Node{NewNode("node001", 0)})

	cache.mu.Lock()
	cache.nodes = nil
	cache.mu.Unlock()

	if _, ok := cache.Get("node001"); ok {
		t.Fatal("cache should not report stale node after being cleared")
	}
	if got := cache.Generation(); got != 1 {
		t.Fatalf("generation = %d, want 1", got)
	}
}
