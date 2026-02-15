package weather

import (
	"sync"
	"time"
)

type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

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
	if !ok || time.Now().After(entry.expiresAt) {
		var zero V
		return zero, false
	}
	return entry.value, true
}

func (c *Cache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cacheEntry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
}
