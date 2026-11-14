package weather

import (
	"sync"
	"time"
)

type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

func (e cacheEntry[V]) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && now.After(e.expiresAt)
}

// Cache is a TTL map. A ttl of zero or less disables expiry; callers then
// evict explicitly with DeleteIf or DeleteFunc. Expired entries are dropped by
// a sweep amortized over Set, so keys that are never read again do not
// accumulate.
type Cache[V any] struct {
	mu      sync.RWMutex
	ttl     time.Duration
	max     int
	m       map[string]cacheEntry[V]
	sweptAt time.Time
}

// Limit caps the cache at max entries and returns it. Adding a key to a full
// cache evicts the expired entries, or failing that the one expiring soonest.
func (c *Cache[V]) Limit(max int) *Cache[V] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.max = max
	return c
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
	if !ok || entry.expired(time.Now()) {
		var zero V
		return zero, false
	}
	return entry.value, true
}

func (c *Cache[V]) Set(key string, value V) {
	c.SetTTL(key, value, c.ttl)
}

// SetTTL stores value under its own time to live instead of the cache's. A
// ttl of zero or less never expires.
func (c *Cache[V]) SetTTL(key string, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	entry := cacheEntry[V]{value: value}
	if ttl > 0 {
		entry.expiresAt = now.Add(ttl)
	}
	if c.ttl > 0 {
		c.sweepLocked(now)
	}
	if _, exists := c.m[key]; !exists && c.max > 0 && len(c.m) >= c.max {
		c.evictLocked(now)
	}
	c.m[key] = entry
}

// evictLocked makes room for one entry in a full cache. The caller holds the
// write lock.
func (c *Cache[V]) evictLocked(now time.Time) {
	var soonest string
	var soonestAt time.Time
	found := false
	for key, entry := range c.m {
		if entry.expired(now) {
			delete(c.m, key)
			continue
		}
		if !found || (!entry.expiresAt.IsZero() && (soonestAt.IsZero() || entry.expiresAt.Before(soonestAt))) {
			soonest, soonestAt, found = key, entry.expiresAt, true
		}
	}
	if found && len(c.m) >= c.max {
		delete(c.m, soonest)
	}
}

// sweepLocked drops every expired entry, at most once per ttl. The caller holds
// the write lock.
func (c *Cache[V]) sweepLocked(now time.Time) {
	if now.Sub(c.sweptAt) < c.ttl {
		return
	}
	c.sweptAt = now
	for key, entry := range c.m {
		if entry.expired(now) {
			delete(c.m, key)
		}
	}
}

// DeleteIf removes every entry whose key satisfies pred.
func (c *Cache[V]) DeleteIf(pred func(key string) bool) {
	c.DeleteFunc(func(key string, _ V) bool { return pred(key) })
}

// DeleteFunc removes every entry that satisfies pred.
func (c *Cache[V]) DeleteFunc(pred func(key string, value V) bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.m {
		if pred(key, entry.value) {
			delete(c.m, key)
		}
	}
}
