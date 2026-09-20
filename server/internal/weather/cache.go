package weather

import (
	"sync"
	"time"
)

type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

// Cache is a TTL map. A ttl of zero or less disables expiry; callers then
// evict explicitly with DeleteIf. Expired entries are dropped by a sweep
// amortized over Set, so keys that are never read again do not accumulate.
type Cache[V any] struct {
	mu      sync.RWMutex
	ttl     time.Duration
	m       map[string]cacheEntry[V]
	sweptAt time.Time
}

func NewCache[V any](ttl time.Duration) *Cache[V] {
	return &Cache[V]{
		ttl:     ttl,
		m:       make(map[string]cacheEntry[V]),
		sweptAt: time.Now(),
	}
}

func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.m[key]
	if !ok || (c.ttl > 0 && time.Now().After(entry.expiresAt)) {
		var zero V
		return zero, false
	}
	return entry.value, true
}

func (c *Cache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := cacheEntry[V]{value: value}
	if c.ttl > 0 {
		now := time.Now()
		entry.expiresAt = now.Add(c.ttl)
		c.sweepLocked(now)
	}
	c.m[key] = entry
}

// sweepLocked drops every expired entry, at most once per ttl. The caller holds
// the write lock.
func (c *Cache[V]) sweepLocked(now time.Time) {
	if now.Sub(c.sweptAt) < c.ttl {
		return
	}
	c.sweptAt = now
	for key, entry := range c.m {
		if now.After(entry.expiresAt) {
			delete(c.m, key)
		}
	}
}

// DeleteIf removes every entry whose key satisfies pred.
func (c *Cache[V]) DeleteIf(pred func(key string) bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.m {
		if pred(key) {
			delete(c.m, key)
		}
	}
}
