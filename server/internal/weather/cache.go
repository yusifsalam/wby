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
// evict explicitly with DeleteIf.
type Cache[V any] struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]cacheEntry[V]
}

func NewCache[V any](ttl time.Duration) *Cache[V] {
	return &Cache[V]{
		ttl: ttl,
		m:   make(map[string]cacheEntry[V]),
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
		entry.expiresAt = time.Now().Add(c.ttl)
	}
	c.m[key] = entry
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
