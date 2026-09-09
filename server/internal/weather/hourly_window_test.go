package weather

import (
	"testing"
	"time"
)

func TestUpcomingHours(t *testing.T) {
	base := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	var hourly []HourlyForecast
	for h := 0; h < 24; h++ {
		hourly = append(hourly, HourlyForecast{Time: base.Add(time.Duration(h) * time.Hour)})
	}

	// 13:40 belongs to the 13:00 slot, which is the first entry returned.
	got := upcomingHours(hourly, base.Add(13*time.Hour+40*time.Minute), 12)
	if len(got) != 11 {
		t.Fatalf("expected the 11 remaining hours, got %d", len(got))
	}
	if !got[0].Time.Equal(base.Add(13 * time.Hour)) {
		t.Errorf("expected first entry at 13:00, got %s", got[0].Time)
	}

	got = upcomingHours(hourly, base.Add(2*time.Hour), 12)
	if len(got) != 12 || !got[0].Time.Equal(base.Add(2*time.Hour)) {
		t.Errorf("expected 12 entries from 02:00, got %d from %v", len(got), got)
	}

	if got := upcomingHours(hourly, base.AddDate(0, 0, 1), 12); len(got) != 0 {
		t.Errorf("expected nothing after the window, got %d", len(got))
	}
	if got := upcomingHours(nil, base, 12); len(got) != 0 {
		t.Errorf("expected nothing for no entries, got %d", len(got))
	}
}
