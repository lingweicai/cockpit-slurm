package resource

// Snapshot is the canonical backend-owned read model for Node data.
type Snapshot struct {
	Generation uint64 `json:"generation"`
	Resource   string `json:"resource"`
	Nodes      []Node `json:"nodes"`
}

// NewSnapshot creates a snapshot for a node collection.
func NewSnapshot(resource string, generation uint64, nodes []Node) Snapshot {
	clone := make([]Node, len(nodes))
	copy(clone, nodes)
	return Snapshot{
		Generation: generation,
		Resource:   resource,
		Nodes:      clone,
	}
}
