package resource

import "sync"

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
	c.mu.Lock()
	defer c.mu.Unlock()

	updated := make(map[string]Node, len(nodes))
	for _, n := range nodes {
		updated[n.Identity()] = n
	}
	c.nodes = updated
	c.generation++
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
