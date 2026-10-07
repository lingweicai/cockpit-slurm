package resource

import "testing"

func TestNodeCacheReplaceSnapshotAndGeneration(t *testing.T) {
	cache := NewNodeCache()

	first := []Node{NewNode("node001", 0)}
	batch, changed := cache.ReplaceSnapshot(first)
	if !changed {
		t.Fatal("first snapshot should change the cache")
	}
	if batch.Generation != 1 || len(batch.Changes) != 1 || batch.Changes[0].Kind != NodeAdded {
		t.Fatalf("first batch = %#v, want one added node at generation 1", batch)
	}
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
	batch, changed = cache.ReplaceSnapshot(second)
	if !changed {
		t.Fatal("replacement snapshot should change the cache")
	}
	if len(batch.Changes) != 2 || batch.Changes[0].Kind != NodeRemoved || batch.Changes[1].Kind != NodeAdded {
		t.Fatalf("replacement changes = %#v, want one removal and one addition", batch.Changes)
	}
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

func TestNodeCacheIgnoresMetadataOnlyRefresh(t *testing.T) {
	cache := NewNodeCache()
	initial := NewNode("node001", 1)
	cache.ReplaceSnapshot([]Node{initial})

	refreshed := initial
	refreshed.Metadata.Generation = 99
	refreshed.Metadata.ObservedAt = refreshed.Metadata.ObservedAt.Add(1)
	refreshed.Metadata.Kind = "Other"
	refreshed.Metadata.Source = "another-source"
	refreshed.Metadata.Name = "other-name"

	batch, changed := cache.ReplaceSnapshot([]Node{refreshed})
	if changed {
		t.Fatalf("metadata-only refresh produced changes: %#v", batch)
	}
	if got := cache.Generation(); got != 1 {
		t.Fatalf("generation = %d, want 1", got)
	}
	node, ok := cache.Get("node001")
	if !ok {
		t.Fatal("expected node001 to remain in cache")
	}
	if node.Metadata.Generation != 1 {
		t.Fatalf("node metadata generation = %d, want 1", node.Metadata.Generation)
	}
}

func TestNodeCacheClassifiesChangesAtOneGeneration(t *testing.T) {
	cache := NewNodeCache()
	initialNode := NewNode("node001", 0)
	initialNode.Spec.RealMemory = 100
	removedNode := NewNode("node002", 0)
	cache.ReplaceSnapshot([]Node{initialNode, removedNode})

	updatedNode := initialNode
	updatedNode.Spec.RealMemory = 200
	addedNode := NewNode("node003", 0)
	batch, changed := cache.ReplaceSnapshot([]Node{updatedNode, addedNode})
	if !changed {
		t.Fatal("changed snapshot did not produce a batch")
	}
	if batch.Resource != "node" || batch.Event != "changes" || batch.Generation != 2 {
		t.Fatalf("batch metadata = %#v, want node changes at generation 2", batch)
	}
	if len(batch.Changes) != 3 {
		t.Fatalf("changes count = %d, want 3", len(batch.Changes))
	}
	want := []struct {
		identity string
		kind     NodeChangeKind
	}{
		{identity: "node001", kind: NodeUpdated},
		{identity: "node002", kind: NodeRemoved},
		{identity: "node003", kind: NodeAdded},
	}
	for i, change := range batch.Changes {
		if change.Node.Identity() != want[i].identity || change.Kind != want[i].kind {
			t.Fatalf("change[%d] = (%q, %q), want (%q, %q)",
				i, change.Node.Identity(), change.Kind, want[i].identity, want[i].kind)
		}
	}
	if batch.Changes[0].Node.Metadata.Generation != batch.Generation {
		t.Fatalf("updated node metadata generation = %d, want %d",
			batch.Changes[0].Node.Metadata.Generation, batch.Generation)
	}
	if batch.Changes[1].Node.Metadata.Generation != 1 {
		t.Fatalf("removed node metadata generation = %d, want previous generation 1",
			batch.Changes[1].Node.Metadata.Generation)
	}
	if batch.Changes[2].Node.Metadata.Generation != batch.Generation {
		t.Fatalf("added node metadata generation = %d, want %d",
			batch.Changes[2].Node.Metadata.Generation, batch.Generation)
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
