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

func TestCache_SetTTLOverridesTheDefault(t *testing.T) {
	c := NewCache[string](time.Hour)
	c.SetTTL("brief", "v", 30*time.Millisecond)
	c.Set("normal", "v")
	if _, ok := c.Get("brief"); !ok {
		t.Fatal("expected a hit before the entry's own TTL passed")
	}

	time.Sleep(60 * time.Millisecond)

	if _, ok := c.Get("brief"); ok {
		t.Error("entry outlived its own TTL")
	}
	if _, ok := c.Get("normal"); !ok {
		t.Error("entry with the default TTL expired early")
	}
}

func TestCache_DeleteFuncSeesValues(t *testing.T) {
	c := NewCache[int](0)
	c.Set("a", 1)
	c.Set("b", 2)
	c.DeleteFunc(func(_ string, v int) bool { return v > 1 })
	if _, ok := c.Get("a"); !ok {
		t.Error("a should remain")
	}
	if _, ok := c.Get("b"); ok {
		t.Error("b should be deleted")
	}
}
