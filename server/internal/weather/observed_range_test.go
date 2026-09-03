package weather

import (
	"errors"
	"testing"
	"time"
)

func TestWidenWithObservedRange(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	today := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	now := today.Add(20 * time.Hour)
	forecast := []DailyForecast{
		{Date: today, TempHigh: f(14), TempLow: f(12)},
		{Date: today.AddDate(0, 0, 1), TempHigh: f(18), TempLow: f(9)},
	}

	var windows [][2]time.Time
	out := widenWithObservedRange(forecast, now, func(from, to time.Time) (*float64, *float64, error) {
		windows = append(windows, [2]time.Time{from, to})
		return f(7.5), f(21), nil
	})

	if len(windows) != 1 || !windows[0][0].Equal(today) || !windows[0][1].Equal(now) {
		t.Fatalf("expected one lookup for [today, now), got %v", windows)
	}
	if *out[0].TempHigh != 21 || *out[0].TempLow != 7.5 {
		t.Errorf("today: got high %v low %v, want 21 / 7.5", *out[0].TempHigh, *out[0].TempLow)
	}
	if *out[1].TempHigh != 18 || *out[1].TempLow != 9 {
		t.Errorf("tomorrow must be untouched, got high %v low %v", *out[1].TempHigh, *out[1].TempLow)
	}
	if *forecast[0].TempHigh != 14 || *forecast[0].TempLow != 12 {
		t.Errorf("input slice must not be mutated, got high %v low %v", *forecast[0].TempHigh, *forecast[0].TempLow)
	}
}

func TestWidenWithObservedRangeKeepsForecastWhenObservedIsNarrower(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	today := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	forecast := []DailyForecast{{Date: today, TempHigh: f(14), TempLow: f(6)}}

	out := widenWithObservedRange(forecast, today.Add(8*time.Hour), func(from, to time.Time) (*float64, *float64, error) {
		return f(9), f(11), nil
	})
	if *out[0].TempHigh != 14 || *out[0].TempLow != 6 {
		t.Errorf("got high %v low %v, want 14 / 6", *out[0].TempHigh, *out[0].TempLow)
	}
}

func TestWidenWithObservedRangePastDayUsesFullWindow(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	yesterday := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	forecast := []DailyForecast{{Date: yesterday, TempHigh: f(14), TempLow: f(12)}}

	var window [2]time.Time
	out := widenWithObservedRange(forecast, yesterday.Add(26*time.Hour), func(from, to time.Time) (*float64, *float64, error) {
		window = [2]time.Time{from, to}
		return nil, nil, nil
	})
	if !window[0].Equal(yesterday) || !window[1].Equal(yesterday.Add(24*time.Hour)) {
		t.Fatalf("expected full-day window, got %v", window)
	}
	if *out[0].TempHigh != 14 || *out[0].TempLow != 12 {
		t.Errorf("nil observations must leave the forecast alone")
	}
}

func TestWidenWithObservedRangeLookupErrorReturnsInput(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	today := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	forecast := []DailyForecast{{Date: today, TempHigh: f(14), TempLow: f(12)}}

	out := widenWithObservedRange(forecast, today.Add(time.Hour), func(from, to time.Time) (*float64, *float64, error) {
		return nil, nil, errors.New("db down")
	})
	if *out[0].TempHigh != 14 || *out[0].TempLow != 12 {
		t.Errorf("got high %v low %v, want forecast unchanged", *out[0].TempHigh, *out[0].TempLow)
	}
}
