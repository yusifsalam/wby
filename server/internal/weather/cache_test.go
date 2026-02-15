package weather

import (
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
