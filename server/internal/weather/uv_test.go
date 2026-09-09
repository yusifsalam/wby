package weather

import (
	"testing"
	"time"
)

func TestStartOfLocalDay(t *testing.T) {
	now := time.Date(2026, 9, 9, 22, 30, 0, 0, time.UTC)

	got := startOfLocalDay(now, "Europe/Helsinki")
	want := time.Date(2026, 9, 9, 21, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("Helsinki: got %s, want %s", got, want)
	}

	got = startOfLocalDay(now, "bogus/zone")
	want = time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("fallback: got %s, want %s", got, want)
	}
}
