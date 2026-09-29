package resource

import (
	"sort"
	"sync"
)

// NodeCache stores the authoritative in-memory snapshot for Nodes.
type NodeCache struct {
	mu       sync.RWMutex
	generation int64
	nodes    map[string]Node
}

func NewNodeCache() *NodeCache {
	return &NodeCache{nodes: make(map[string]Node)}
}

func (c *NodeCache) ReplaceSnapshot(nodes []Node) {
	c.ReplaceSnapshotIfChanged(nodes)
}

// ReplaceSnapshotIfChanged commits a new complete Node collection when its
// semantic state differs from the cached collection.
func (c *NodeCache) ReplaceSnapshotIfChanged(nodes []Node) (bool, int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	updated := make(map[string]Node, len(nodes))
	for _, n := range nodes {
		updated[n.Identity()] = n
	}
	if nodeCollectionsEqual(c.nodes, updated) {
		return false, c.generation
	}
	c.nodes = updated
	c.generation++
	return true, c.generation
}

func nodeCollectionsEqual(current, updated map[string]Node) bool {
	if len(current) != len(updated) {
		return false
	}
	for identity, currentNode := range current {
		updatedNode, ok := updated[identity]
		if !ok || !semanticallyEqual(currentNode, updatedNode) {
			return false
		}
	}
	return true
}

func semanticallyEqual(current, updated Node) bool {
	currentFlags := append([]string(nil), current.Status.StateFlags...)
	updatedFlags := append([]string(nil), updated.Status.StateFlags...)
	sort.Strings(currentFlags)
	sort.Strings(updatedFlags)

	return current.Identity() == updated.Identity() &&
		current.Spec == updated.Spec &&
		current.Status.State == updated.Status.State &&
		current.Status.Reason == updated.Status.Reason &&
		len(currentFlags) == len(updatedFlags) &&
		stringSlicesEqual(currentFlags, updatedFlags)
}

func stringSlicesEqual(left, right []string) bool {
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (c *NodeCache) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := make([]Node, 0, len(c.nodes))
	for _, n := range c.nodes {
		items = append(items, n)
	}
	return NewSnapshot("nodes", c.generation, items)
}

func (c *NodeCache) Get(name string) (Node, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	n, ok := c.nodes[name]
	return n, ok
}

func (c *NodeCache) Generation() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.generation
}
