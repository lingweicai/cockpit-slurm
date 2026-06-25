package entity

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// EntityProvider loads entity records from Slurm.
type EntityProvider[T any] interface {
	List(ctx context.Context) ([]*T, error)
}

// EntityCache stores entity records in memory.
type EntityCache[T any] struct {
	mu         sync.RWMutex
	generation uint64
	updatedAt  time.Time
	entities   map[string]*T
	keyFn      func(*T) string
	cloneFn    func(*T) *T
}

// EntityDiff represents changes between two entity snapshots.
type EntityDiff[T any] struct {
	Added    []*T
	Modified []*T
	Deleted  []*T
}

// EntityEvent is the broadcast payload for a cache refresh.
type EntityEvent[T any] struct {
	Entity     string `json:"entity"`
	Generation uint64 `json:"generation"`
	Added      []*T   `json:"added,omitempty"`
	Modified   []*T   `json:"modified,omitempty"`
	Deleted    []*T   `json:"deleted,omitempty"`
}

// EntityManager coordinates provider refreshes and in-memory caching.
type EntityManager[T any] struct {
	provider EntityProvider[T]
	cache    *EntityCache[T]
	equalFn  func(*T, *T) bool
	entity   string
}

// NewEntityCache creates an empty cache for the given entity type.
func NewEntityCache[T any](keyFn func(*T) string, cloneFn func(*T) *T) *EntityCache[T] {
	return &EntityCache[T]{
		entities: make(map[string]*T),
		keyFn:    keyFn,
		cloneFn:  cloneFn,
	}
}

// NewEntityManager creates an entity manager with the provided hooks.
func NewEntityManager[T any](entity string, provider EntityProvider[T], keyFn func(*T) string, cloneFn func(*T) *T, equalFn func(*T, *T) bool) *EntityManager[T] {
	return &EntityManager[T]{
		provider: provider,
		cache:    NewEntityCache(keyFn, cloneFn),
		equalFn:  equalFn,
		entity:   entity,
	}
}

// Cache returns the backing cache.
func (m *EntityManager[T]) Cache() *EntityCache[T] {
	if m == nil {
		return nil
	}
	return m.cache
}

// LoadInitialCache populates the cache from the provider.
func (m *EntityManager[T]) LoadInitialCache(ctx context.Context) (int, error) {
	if m == nil {
		return 0, fmt.Errorf("entity manager is nil")
	}
	if m.provider == nil {
		return 0, fmt.Errorf("entity provider is nil")
	}

	entities, err := m.provider.List(ctx)
	if err != nil {
		return 0, err
	}

	m.cache.ReplaceAll(entities)
	return len(entities), nil
}

// Snapshot returns the cache contents and generation.
func (m *EntityManager[T]) Snapshot() ([]*T, uint64) {
	if m == nil || m.cache == nil {
		return nil, 0
	}
	return m.cache.List(), m.cache.Generation()
}

// RefreshOnce reloads the provider and updates the cache if needed.
func (m *EntityManager[T]) RefreshOnce(ctx context.Context) (EntityDiff[T], error) {
	if m == nil {
		return EntityDiff[T]{}, fmt.Errorf("entity manager is nil")
	}
	if m.provider == nil {
		return EntityDiff[T]{}, fmt.Errorf("entity provider is nil")
	}

	current := m.cache.AsMap()
	entities, err := m.provider.List(ctx)
	if err != nil {
		return EntityDiff[T]{}, err
	}

	next := make(map[string]*T, len(entities))
	for _, entity := range entities {
		if entity == nil {
			continue
		}
		key := m.cache.key(entity)
		if key == "" {
			continue
		}
		next[key] = entity
	}

	diff := Compare(current, next, m.equalFn, m.cache.clone, m.cache.key)
	if diff.Empty() {
		return diff, nil
	}

	m.cache.ReplaceAll(entities)
	return diff, nil
}

// Run polls the provider on the given interval until the context is done.
func (m *EntityManager[T]) Run(ctx context.Context, interval time.Duration) error {
	if m == nil {
		return fmt.Errorf("entity manager is nil")
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := m.RefreshOnce(ctx); err != nil {
				return err
			}
		}
	}
}

// Compare computes a diff between two entity maps.
func Compare[T any](oldMap, newMap map[string]*T, equalFn func(*T, *T) bool, cloneFn func(*T) *T, keyFn func(*T) string) EntityDiff[T] {
	diff := EntityDiff[T]{}
	if equalFn == nil {
		equalFn = func(a, b *T) bool { return a == b }
	}

	for key, entity := range newMap {
		oldEntity, ok := oldMap[key]
		if !ok {
			diff.Added = append(diff.Added, cloneEntity(entity, cloneFn))
			continue
		}
		if !equalFn(oldEntity, entity) {
			diff.Modified = append(diff.Modified, cloneEntity(entity, cloneFn))
		}
	}

	for key, entity := range oldMap {
		if _, ok := newMap[key]; !ok {
			diff.Deleted = append(diff.Deleted, cloneEntity(entity, cloneFn))
		}
	}

	sortEntitiesByKey(diff.Added, keyFn)
	sortEntitiesByKey(diff.Modified, keyFn)
	sortEntitiesByKey(diff.Deleted, keyFn)

	return diff
}

// Empty reports whether the diff contains any changes.
func (d EntityDiff[T]) Empty() bool {
	return len(d.Added) == 0 && len(d.Modified) == 0 && len(d.Deleted) == 0
}

// Set inserts or replaces an entity.
func (c *EntityCache[T]) Set(entity *T) {
	if c == nil || entity == nil {
		return
	}

	key := c.key(entity)
	if key == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureLocked()
	c.entities[key] = c.clone(entity)
	c.generation++
	c.updatedAt = time.Now().UTC()
}

// Delete removes an entity by key.
func (c *EntityCache[T]) Delete(key string) {
	if c == nil || key == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.entities[key]; !ok {
		return
	}

	delete(c.entities, key)
	c.generation++
	c.updatedAt = time.Now().UTC()
}

// ReplaceAll swaps the cache contents.
func (c *EntityCache[T]) ReplaceAll(entities []*T) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entities = make(map[string]*T, len(entities))
	for _, entity := range entities {
		if entity == nil {
			continue
		}
		key := c.key(entity)
		if key == "" {
			continue
		}
		c.entities[key] = c.clone(entity)
	}
	c.generation++
	c.updatedAt = time.Now().UTC()
}

// Get returns a cloned entity by key.
func (c *EntityCache[T]) Get(key string) (*T, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entity, ok := c.entities[key]
	if !ok {
		return nil, false
	}
	return c.clone(entity), true
}

// List returns all entities sorted by key.
func (c *EntityCache[T]) List() []*T {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.entities) == 0 {
		return nil
	}

	keys := make([]string, 0, len(c.entities))
	for key := range c.entities {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	entities := make([]*T, 0, len(keys))
	for _, key := range keys {
		entities = append(entities, c.clone(c.entities[key]))
	}
	return entities
}

// AsMap returns a cloned map keyed by entity ID.
func (c *EntityCache[T]) AsMap() map[string]*T {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.entities) == 0 {
		return map[string]*T{}
	}

	out := make(map[string]*T, len(c.entities))
	for key, entity := range c.entities {
		out[key] = c.clone(entity)
	}
	return out
}

// Generation returns the current generation.
func (c *EntityCache[T]) Generation() uint64 {
	if c == nil {
		return 0
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.generation
}

// UpdatedAt returns the last update time.
func (c *EntityCache[T]) UpdatedAt() time.Time {
	if c == nil {
		return time.Time{}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.updatedAt
}

func (c *EntityCache[T]) key(entity *T) string {
	if c == nil || c.keyFn == nil || entity == nil {
		return ""
	}
	return c.keyFn(entity)
}

func (c *EntityCache[T]) clone(entity *T) *T {
	if c == nil {
		return nil
	}
	if c.cloneFn == nil {
		return entity
	}
	return c.cloneFn(entity)
}

func (c *EntityCache[T]) ensureLocked() {
	if c.entities == nil {
		c.entities = make(map[string]*T)
	}
}

func cloneEntity[T any](entity *T, cloneFn func(*T) *T) *T {
	if cloneFn == nil {
		return entity
	}
	return cloneFn(entity)
}

func sortEntitiesByKey[T any](entities []*T, keyFn func(*T) string) {
	sort.Slice(entities, func(i, j int) bool {
		if entities[i] == nil {
			return true
		}
		if entities[j] == nil {
			return false
		}
		if keyFn == nil {
			return false
		}
		return keyFn(entities[i]) < keyFn(entities[j])
	})
}
