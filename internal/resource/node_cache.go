package resource

import "sync"

// NodeCache stores the authoritative in-memory snapshot for Nodes.
type NodeCache struct {
	publicationMu sync.Mutex
	mu            sync.RWMutex
	generation    uint64
	nodes         map[string]Node
}

func NewNodeCache() *NodeCache {
	return &NodeCache{nodes: make(map[string]Node)}
}

// ReplaceSnapshot commits a complete Node snapshot and returns the changes
// committed at the new generation. Unchanged snapshots do not advance the
// generation and return changed=false.
func (c *NodeCache) ReplaceSnapshot(nodes []Node) (batch NodeChangeBatch, changed bool) {
	c.publicationMu.Lock()
	defer c.publicationMu.Unlock()

	return c.replaceSnapshot(nodes)
}

// ReplaceSnapshotAndPublish keeps cache commits and change publication ordered
// with stream baseline registration.
func (c *NodeCache) ReplaceSnapshotAndPublish(nodes []Node, publisher NodeChangePublisher) (batch NodeChangeBatch, changed bool) {
	c.publicationMu.Lock()
	defer c.publicationMu.Unlock()

	batch, changed = c.replaceSnapshot(nodes)
	if changed && publisher != nil {
		publisher.Publish(batch)
	}
	return batch, changed
}

// WithSnapshotBoundary captures a snapshot while excluding cache commits and
// publication until the callback has established a stream baseline.
func (c *NodeCache) WithSnapshotBoundary(register func(Snapshot)) {
	c.publicationMu.Lock()
	defer c.publicationMu.Unlock()
	register(c.Snapshot())
}

func (c *NodeCache) replaceSnapshot(nodes []Node) (batch NodeChangeBatch, changed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	incoming := make(map[string]Node, len(nodes))
	for _, n := range nodes {
		incoming[n.Identity()] = n
	}

	var changes []NodeChange
	for identity, node := range incoming {
		old, exists := c.nodes[identity]
		if !exists {
			changes = append(changes, NodeChange{Kind: NodeAdded, Node: node})
			continue
		}
		if !sameNodeState(old, node) {
			changes = append(changes, NodeChange{Kind: NodeUpdated, Node: node})
		}
	}
	for identity, node := range c.nodes {
		if _, exists := incoming[identity]; !exists {
			changes = append(changes, NodeChange{Kind: NodeRemoved, Node: node})
		}
	}

	if len(changes) == 0 {
		return NodeChangeBatch{}, false
	}

	c.generation++

	updated := make(map[string]Node, len(incoming))
	for identity, node := range incoming {
		if old, exists := c.nodes[identity]; exists && sameNodeState(old, node) {
			updated[identity] = old
			continue
		}
		node.Metadata.Generation = c.generation
		updated[identity] = node
	}
	for i := range changes {
		if changes[i].Kind != NodeRemoved {
			changes[i].Node.Metadata.Generation = c.generation
		}
	}
	sortNodeChanges(changes)
	c.nodes = updated

	return NodeChangeBatch{
		Resource:   "node",
		Event:      "changes",
		Generation: c.generation,
		Changes:    changes,
	}, true
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

func (c *NodeCache) Generation() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.generation
}
