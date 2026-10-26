package resource

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

const DefaultNodeSyncInterval = 5 * time.Second

type NodeChangePublisher interface {
	Publish(batch NodeChangeBatch)
}

// NodeSynchronizer refreshes the authoritative Node cache independently of
// connected clients.
type NodeSynchronizer struct {
	adapter   NodeAdapter
	cache     *NodeCache
	publisher NodeChangePublisher
	interval  time.Duration

	refreshMu sync.Mutex
}

func NewNodeSynchronizer(adapter NodeAdapter, cache *NodeCache, publisher NodeChangePublisher, interval time.Duration) (*NodeSynchronizer, error) {
	if adapter == nil {
		return nil, errors.New("node adapter is required")
	}
	if cache == nil {
		return nil, errors.New("node cache is required")
	}
	if interval <= 0 {
		return nil, fmt.Errorf("node synchronization interval must be positive: %s", interval)
	}

	return &NodeSynchronizer{
		adapter:   adapter,
		cache:     cache,
		publisher: publisher,
		interval:  interval,
	}, nil
}

// Refresh reads and validates a complete Node snapshot before committing it.
// A failed refresh leaves the current cache and generation unchanged.
func (s *NodeSynchronizer) Refresh(ctx context.Context) error {
	if s == nil || s.adapter == nil || s.cache == nil {
		return errors.New("node synchronizer is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	nodes, err := s.adapter.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("list Slurm nodes: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateNodeSnapshot(nodes); err != nil {
		return fmt.Errorf("validate Slurm node snapshot: %w", err)
	}

	s.cache.ReplaceSnapshotAndPublish(nodes, s.publisher)
	return nil
}

// Run refreshes the cache periodically. Call Refresh first when the initial
// snapshot must be committed before serving clients.
func (s *NodeSynchronizer) Run(ctx context.Context) error {
	if s == nil || s.interval <= 0 {
		return errors.New("node synchronizer is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.Refresh(ctx); err != nil && ctx.Err() == nil {
				log.Printf("Node synchronization failed: %v", err)
			}
		}
	}
}

func validateNodeSnapshot(nodes []Node) error {
	identities := make(map[string]struct{}, len(nodes))
	for i, node := range nodes {
		identity := node.Identity()
		if identity == "" {
			return fmt.Errorf("node at index %d has no identity", i)
		}
		if _, exists := identities[identity]; exists {
			return fmt.Errorf("duplicate node identity %q", identity)
		}
		identities[identity] = struct{}{}
	}
	return nil
}
