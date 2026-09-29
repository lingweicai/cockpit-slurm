package resource

import (
	"context"
	"fmt"
	"time"
)

// NodeSynchronizer refreshes the authoritative Node cache independently of
// frontend connections.
type NodeSynchronizer struct {
	adapter   NodeAdapter
	cache     *NodeCache
	interval  time.Duration
	onChanged func(Snapshot)
}

func NewNodeSynchronizer(adapter NodeAdapter, cache *NodeCache, interval time.Duration, onChanged func(Snapshot)) (*NodeSynchronizer, error) {
	if adapter == nil {
		return nil, fmt.Errorf("node adapter is required")
	}
	if cache == nil {
		return nil, fmt.Errorf("node cache is required")
	}
	if interval <= 0 {
		return nil, fmt.Errorf("synchronization interval must be positive")
	}
	return &NodeSynchronizer{
		adapter:   adapter,
		cache:     cache,
		interval:  interval,
		onChanged: onChanged,
	}, nil
}

// Refresh performs one synchronization cycle. A failed refresh leaves the
// last valid cache state untouched.
func (s *NodeSynchronizer) Refresh(ctx context.Context) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	nodes, err := s.adapter.ListNodes(ctx)
	if err != nil {
		return false, err
	}
	changed, _ := s.cache.ReplaceSnapshotIfChanged(nodes)
	if changed && s.onChanged != nil {
		s.onChanged(s.cache.Snapshot())
	}
	return changed, nil
}

// RunPeriodic refreshes the cache until the context is canceled. Periodic
// refresh failures are retained as errors only for the caller's diagnostics;
// they do not stop future refreshes or alter the cache.
func (s *NodeSynchronizer) RunPeriodic(ctx context.Context, onError func(error)) {
	if ctx == nil {
		ctx = context.Background()
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.Refresh(ctx); err != nil && onError != nil {
				onError(err)
			}
		}
	}
}