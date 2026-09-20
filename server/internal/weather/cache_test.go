package weather

import (
	"fmt"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	c := NewCache[string](1 * time.Second)
	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}
}

func TestCache_Expiry(t *testing.T) {
	c := NewCache[string](50 * time.Millisecond)
	c.Set("key1", "value1")

	time.Sleep(100 * time.Millisecond)

	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected cache miss after TTL")
	}
}

func TestCache_SweepDropsKeysNeverReadAgain(t *testing.T) {
	c := NewCache[string](50 * time.Millisecond)
	for i := range 500 {
		c.Set(fmt.Sprintf("rotating-%d", i), "v")
	}
	if got := cacheLen(c); got != 500 {
		t.Fatalf("cache holds %d entries, want 500", got)
	}

	time.Sleep(100 * time.Millisecond)

	// One write by a live key; the 500 expired keys are never read again.
	c.Set("live", "v")
	if got := cacheLen(c); got != 1 {
		t.Fatalf("cache holds %d entries after sweep, want 1", got)
	}
	if _, ok := c.Get("live"); !ok {
		t.Fatal("sweep dropped the entry written alongside it")
	}
}

func TestCache_SweepRunsAtMostOncePerTTL(t *testing.T) {
	c := NewCache[string](time.Hour)
	c.Set("a", "v")
	c.Set("b", "v")
	if got := cacheLen(c); got != 2 {
		t.Fatalf("cache holds %d entries, want 2", got)
	}
}

func TestCache_ZeroTTLKeepsEntriesForExplicitPruning(t *testing.T) {
	c := NewCache[string](0)
	c.Set("warm", "v")

	time.Sleep(10 * time.Millisecond)
	c.Set("other", "v")

	if _, ok := c.Get("warm"); !ok {
		t.Fatal("zero-TTL entry expired; warm caches rely on DeleteIf only")
	}
	c.DeleteIf(func(k string) bool { return k == "warm" })
	if _, ok := c.Get("warm"); ok {
		t.Fatal("DeleteIf did not remove the entry")
	}
}
