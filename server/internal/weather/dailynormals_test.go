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
		if d.Year()%2 == 0 {
			precip = -1
		}
		snow := -1.0
		if d.Month() <= 2 || d.Month() == 12 {
			snow = 10
		}
		daily = append(daily, DailyRecord{Date: d, TempAvg: f(base), TempHigh: f(base + 4), TempLow: f(base - 4), PrecipMm: f(precip), SnowCm: f(snow)})
		for h := 0; h < 24; h++ {
			hourly = append(hourly, HourlyRecord{
				Time:      d.Add(time.Duration(h) * time.Hour),
				Temp:      f(base + syntheticHourlyOffset(h)),
				Humidity:  f(80 - 2*syntheticHourlyOffset(h)),
				WindSpeed: f(5),
				WindGust:  f(8 + float64(h%3)),
			})
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
	if y := normals[0].DailyYears; math.Abs(y-30) > 0.1 {
		t.Errorf("DailyYears = %.2f, want ~30 (the 1980 row is outside the period)", y)
	}
	if y := normals[0].HourlyYears; y <= 0 || y > 30.1 {
		t.Errorf("HourlyYears = %.2f, want within (0, 30]", y)
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
		if len(n.TempHourlyP10) != 24 || len(n.TempHourlyP90) != 24 {
			t.Fatalf("%02d-%02d hourly percentiles len = %d/%d", probe.month, probe.day, len(n.TempHourlyP10), len(n.TempHourlyP90))
		}
		for h := 0; h < 24; h++ {
			if n.TempHourlyP10[h] > n.TempHourly[h] || n.TempHourly[h] > n.TempHourlyP90[h] {
				t.Errorf("%02d-%02d hour %d: p10 %.2f, mean %.2f, p90 %.2f out of order", probe.month, probe.day, h, n.TempHourlyP10[h], n.TempHourly[h], n.TempHourlyP90[h])
			}
		}
		if spread := n.TempHourlyP90[12] - n.TempHourlyP10[12]; spread <= 0 || spread > 3 {
			t.Errorf("%02d-%02d noon p10..p90 spread = %.2f, want within the window's seasonal drift", probe.month, probe.day, spread)
		}
		if n.PrecipDaysPct == nil || math.Abs(*n.PrecipDaysPct-50) > 2 {
			t.Errorf("%02d-%02d wet days = %v, want ~50%%", probe.month, probe.day, n.PrecipDaysPct)
		}
		if n.WindAvg == nil || math.Abs(*n.WindAvg-5) > 1e-6 {
			t.Errorf("%02d-%02d wind = %v, want 5", probe.month, probe.day, n.WindAvg)
		}
		if n.WindGust == nil || math.Abs(*n.WindGust-10) > 1e-6 {
			t.Errorf("%02d-%02d gust = %v, want 10 (daily max)", probe.month, probe.day, n.WindGust)
		}
		if n.HumidityAvg == nil || math.Abs(*n.HumidityAvg-80) > 0.3 {
			t.Errorf("%02d-%02d humidity = %v, want ~80", probe.month, probe.day, n.HumidityAvg)
		}
		for name, curve := range map[string][]float64{"feels": n.FeelsLikeHourly, "wind": n.WindHourly, "humidity": n.HumidityHourly} {
			if len(curve) != 24 {
				t.Errorf("%02d-%02d %s hourly len = %d", probe.month, probe.day, name, len(curve))
			}
		}
		if n.FeelsLikeAvg == nil || n.FeelsLikeHigh == nil || n.FeelsLikeLow == nil {
			t.Fatalf("%02d-%02d feels-like nil: %v %v %v", probe.month, probe.day, n.FeelsLikeAvg, n.FeelsLikeHigh, n.FeelsLikeLow)
		}
		if *n.FeelsLikeLow > *n.FeelsLikeAvg || *n.FeelsLikeAvg > *n.FeelsLikeHigh {
			t.Errorf("%02d-%02d feels-like low/avg/high out of order: %.2f %.2f %.2f", probe.month, probe.day, *n.FeelsLikeLow, *n.FeelsLikeAvg, *n.FeelsLikeHigh)
		}
	}

	jan := findNormal(t, normals, 1, 15)
	if *jan.FeelsLikeAvg > *jan.TempAvg-3 {
		t.Errorf("January feels-like %.2f should sit well below %.2f at 5 m/s", *jan.FeelsLikeAvg, *jan.TempAvg)
	}
	if jan.SnowCm == nil || math.Abs(*jan.SnowCm-10) > 1e-6 {
		t.Errorf("January snow = %v, want 10", jan.SnowCm)
	}
	jul := findNormal(t, normals, 7, 15)
	if math.Abs(*jul.FeelsLikeAvg-*jul.TempAvg) > 0.1 {
		t.Errorf("July feels-like %.2f should match %.2f above 10°C", *jul.FeelsLikeAvg, *jul.TempAvg)
	}
	if jul.SnowCm == nil || *jul.SnowCm != 0 {
		t.Errorf("July snow = %v, want 0 (no-snow marker counts as zero)", jul.SnowCm)
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
	if n.TempHourly != nil || n.TempHourlyP10 != nil || n.TempHourlyP90 != nil || n.WindHourly != nil || n.FeelsLikeHourly != nil || n.HumidityHourly != nil {
		t.Errorf("no hourly data, hourly curves should be nil")
	}
	if n.WindAvg != nil || n.FeelsLikeAvg != nil || n.FeelsLikeHigh != nil || n.WindGust != nil || n.HumidityAvg != nil {
		t.Errorf("no hourly data, hourly-derived normals should be nil")
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

func TestComputeDailyNormalsHourlyThresholdIsTenYears(t *testing.T) {
	daily, _ := syntheticRecords(1991, 2020)
	_, twelve := syntheticRecords(2009, 2020)
	_, eight := syntheticRecords(2013, 2020)

	normals, err := ComputeDailyNormals(1, "1991-2020", daily, twelve)
	if err != nil {
		t.Fatal(err)
	}
	n := findNormal(t, normals, 7, 15)
	if n.TempHourly == nil || n.FeelsLikeAvg == nil || n.WindAvg == nil || n.FeelsLikeHigh == nil || n.WindGust == nil {
		t.Errorf("12 of 30 hourly years should yield curves and hourly-derived fields: %+v", n)
	}

	normals, err = ComputeDailyNormals(1, "1991-2020", daily, eight)
	if err != nil {
		t.Fatal(err)
	}
	n = findNormal(t, normals, 7, 15)
	if n.TempHourly != nil || n.FeelsLikeAvg != nil || n.WindAvg != nil || n.FeelsLikeHigh != nil || n.WindGust != nil {
		t.Errorf("8 of 30 hourly years should leave hourly-derived fields nil: %+v", n)
	}
	if n.TempAvg == nil {
		t.Error("daily temperature should be unaffected by the hourly threshold")
	}
}
