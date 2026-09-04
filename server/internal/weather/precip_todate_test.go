package weather

import (
	"math"
	"testing"
	"time"
)

func TestPrecipitationToDate(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Helsinki")
	if err != nil {
		t.Fatal(err)
	}
	f := func(v float64) *float64 { return &v }

	var normals []DailyClimateNormal
	for i := 0; i < calendarDays; i++ {
		m, d := calendarDate(i)
		normals = append(normals, DailyClimateNormal{Month: m, Day: d, PrecipMm: f(2)})
	}

	now := time.Date(2026, 9, 4, 15, 30, 0, 0, loc)
	dayStart := time.Date(2026, 9, 4, 0, 0, 0, 0, loc)
	monthStart := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	var hourly []HourlyRecord
	for t := monthStart.Add(-time.Hour); !t.After(now.Add(2 * time.Hour)); t = t.Add(time.Hour) {
		hourly = append(hourly, HourlyRecord{Time: t.UTC(), PrecipMm: f(0.5)})
	}
	hourly = append(hourly, HourlyRecord{Time: dayStart.Add(-30 * time.Minute).UTC(), PrecipMm: f(-1)})

	got := precipitationToDate(normals, hourly, now, loc)

	hoursToday := 15.0
	if got.TodayObservedMm == nil || math.Abs(*got.TodayObservedMm-hoursToday*0.5) > 1e-9 {
		t.Errorf("today observed = %v, want %.1f (hours 01..15 today)", got.TodayObservedMm, hoursToday*0.5)
	}
	hoursMonth := 3*24 + hoursToday
	if got.MonthToDateObservedMm == nil || math.Abs(*got.MonthToDateObservedMm-hoursMonth*0.5) > 1e-9 {
		t.Errorf("month observed = %v, want %.1f (boundary and future hours excluded, negatives as zero)", got.MonthToDateObservedMm, hoursMonth*0.5)
	}
	if got.ObservedThrough == nil || !got.ObservedThrough.Equal(time.Date(2026, 9, 4, 15, 0, 0, 0, loc)) {
		t.Errorf("observed through = %v, want 15:00 local", got.ObservedThrough)
	}
	if got.TodayNormalMm == nil || *got.TodayNormalMm != 2 {
		t.Errorf("today normal = %v, want 2", got.TodayNormalMm)
	}
	if got.MonthToDateNormalMm == nil || *got.MonthToDateNormalMm != 8 {
		t.Errorf("month-to-date normal = %v, want 8 (4 days)", got.MonthToDateNormalMm)
	}
	if got.MonthNormalMm == nil || *got.MonthNormalMm != 60 {
		t.Errorf("month normal = %v, want 60 (30 days)", got.MonthNormalMm)
	}

	empty := precipitationToDate(normals, nil, now, loc)
	if empty.TodayObservedMm != nil || empty.MonthToDateObservedMm != nil || empty.ObservedThrough != nil {
		t.Errorf("no observations should leave observed values nil: %+v", empty)
	}

	feb := precipitationToDate(normals, nil, time.Date(2026, 2, 10, 12, 0, 0, 0, loc), loc)
	if feb.MonthNormalMm == nil || *feb.MonthNormalMm != 56 {
		t.Errorf("February 2026 month normal = %v, want 56 (29 Feb skipped in a common year)", feb.MonthNormalMm)
	}
}
