package weather

import (
	"math"
	"testing"
	"time"
)

func syntheticDailyTemp(t time.Time) float64 {
	return 5 - 12*math.Cos(2*math.Pi*float64(t.YearDay()-15)/365.25)
}

func syntheticHourlyOffset(hour int) float64 {
	return 3 * math.Sin(2*math.Pi*float64(hour-9)/24)
}

func syntheticRecords(startYear, endYear int) ([]DailyRecord, []HourlyRecord) {
	f := func(v float64) *float64 { return &v }
	var daily []DailyRecord
	var hourly []HourlyRecord
	for d := time.Date(startYear, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() <= endYear; d = d.AddDate(0, 0, 1) {
		base := syntheticDailyTemp(d)
		precip := 2.0
		if d.YearDay()%2 == 0 {
			precip = -1
		}
		daily = append(daily, DailyRecord{Date: d, TempAvg: f(base), TempHigh: f(base + 4), TempLow: f(base - 4), PrecipMm: f(precip)})
		for h := 0; h < 24; h++ {
			hourly = append(hourly, HourlyRecord{Time: d.Add(time.Duration(h) * time.Hour), Temp: base + syntheticHourlyOffset(h)})
		}
	}
	return daily, hourly
}

func findNormal(t *testing.T, normals []DailyClimateNormal, month, day int) DailyClimateNormal {
	t.Helper()
	for _, n := range normals {
		if n.Month == month && n.Day == day {
			return n
		}
	}
	t.Fatalf("no normal for %02d-%02d", month, day)
	return DailyClimateNormal{}
}

func TestComputeDailyNormalsRecoversSeasonalCurve(t *testing.T) {
	daily, hourly := syntheticRecords(1991, 2020)
	f := func(v float64) *float64 { return &v }
	daily = append(daily, DailyRecord{Date: time.Date(1980, 7, 15, 0, 0, 0, 0, time.UTC), TempAvg: f(100), TempHigh: f(100), TempLow: f(100)})

	normals, err := ComputeDailyNormals(100971, "1991-2020", daily, hourly)
	if err != nil {
		t.Fatal(err)
	}
	if len(normals) != 366 {
		t.Fatalf("got %d normals, want 366", len(normals))
	}

	for _, probe := range []struct{ month, day int }{{1, 15}, {4, 1}, {7, 15}, {10, 31}, {12, 31}} {
		n := findNormal(t, normals, probe.month, probe.day)
		want := syntheticDailyTemp(time.Date(2001, time.Month(probe.month), probe.day, 0, 0, 0, 0, time.UTC))
		if n.TempAvg == nil || math.Abs(*n.TempAvg-want) > 0.3 {
			t.Errorf("%02d-%02d avg = %v, want ~%.2f", probe.month, probe.day, n.TempAvg, want)
		}
		if n.TempHigh == nil || math.Abs(*n.TempHigh-(want+4)) > 0.3 {
			t.Errorf("%02d-%02d high = %v, want ~%.2f", probe.month, probe.day, n.TempHigh, want+4)
		}
		if n.TempLow == nil || math.Abs(*n.TempLow-(want-4)) > 0.3 {
			t.Errorf("%02d-%02d low = %v, want ~%.2f", probe.month, probe.day, n.TempLow, want-4)
		}
		if n.PrecipMm == nil || math.Abs(*n.PrecipMm-1) > 0.1 {
			t.Errorf("%02d-%02d precip = %v, want ~1 (dry-day marker counts as zero)", probe.month, probe.day, n.PrecipMm)
		}
		if len(n.TempHourly) != 24 {
			t.Fatalf("%02d-%02d hourly len = %d", probe.month, probe.day, len(n.TempHourly))
		}
		if diff := n.TempHourly[15] - n.TempHourly[3]; math.Abs(diff-6) > 0.2 {
			t.Errorf("%02d-%02d hourly swing = %.2f, want ~6", probe.month, probe.day, diff)
		}
		if math.Abs(n.TempHourly[9]-*n.TempAvg) > 0.3 {
			t.Errorf("%02d-%02d hour 9 = %.2f, want ~daily mean %.2f", probe.month, probe.day, n.TempHourly[9], *n.TempAvg)
		}
	}

	leap := findNormal(t, normals, 2, 29)
	feb28 := findNormal(t, normals, 2, 28)
	mar1 := findNormal(t, normals, 3, 1)
	if leap.TempAvg == nil || *leap.TempAvg < *feb28.TempAvg || *leap.TempAvg > *mar1.TempAvg {
		t.Errorf("29 Feb avg %v should sit between 28 Feb %v and 1 Mar %v", leap.TempAvg, *feb28.TempAvg, *mar1.TempAvg)
	}
}

func TestComputeDailyNormalsLeavesSparseDaysNil(t *testing.T) {
	daily, hourly := syntheticRecords(1991, 2020)
	var partial []DailyRecord
	for _, r := range daily {
		if r.Date.Year() <= 2000 {
			partial = append(partial, r)
		}
	}
	normals, err := ComputeDailyNormals(1, "1991-2020", partial, hourly[:0])
	if err != nil {
		t.Fatal(err)
	}
	n := findNormal(t, normals, 7, 15)
	if n.TempAvg != nil {
		t.Errorf("only 10 of 30 years present, avg should be nil, got %v", *n.TempAvg)
	}
	if n.TempHourly != nil {
		t.Errorf("no hourly data, hourly should be nil")
	}
}

func TestComputeDailyNormalsRejectsBadPeriod(t *testing.T) {
	if _, err := ComputeDailyNormals(1, "1991", nil, nil); err == nil {
		t.Error("expected error for malformed period")
	}
	if _, err := ComputeDailyNormals(1, "2020-1991", nil, nil); err == nil {
		t.Error("expected error for reversed period")
	}
}

func TestHourlyNormalAt(t *testing.T) {
	hours := make([]float64, 24)
	for h := range hours {
		hours[h] = float64(h)
	}
	next := make([]float64, 24)
	next[0] = 30

	got := HourlyNormalAt(hours, nil, time.Date(2026, 9, 3, 10, 30, 0, 0, time.UTC))
	if got == nil || math.Abs(*got-10.5) > 1e-9 {
		t.Errorf("10:30 = %v, want 10.5", got)
	}
	got = HourlyNormalAt(hours, next, time.Date(2026, 9, 3, 23, 30, 0, 0, time.UTC))
	if got == nil || math.Abs(*got-26.5) > 1e-9 {
		t.Errorf("23:30 with next day = %v, want 26.5", got)
	}
	got = HourlyNormalAt(hours, nil, time.Date(2026, 9, 3, 23, 30, 0, 0, time.UTC))
	if got == nil || math.Abs(*got-11.5) > 1e-9 {
		t.Errorf("23:30 wrapping = %v, want 11.5", got)
	}
	if HourlyNormalAt(nil, nil, time.Now()) != nil {
		t.Error("nil curve must yield nil")
	}
}
