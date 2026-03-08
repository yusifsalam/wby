package weather

import (
	"math"
	"testing"
	"time"
)

func ptr(f float64) *float64 { return &f }

func TestInterpolateNormals_MidMonth(t *testing.T) {
	// On Jan 15 (mid-month), the interpolated value should equal January's value exactly
	normals := make([]ClimateNormal, 12)
	for i := range normals {
		v := float64(i + 1) // Jan=1, Feb=2, ..., Dec=12
		normals[i] = ClimateNormal{Month: i + 1, TempAvg: &v}
	}

	date := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	result := InterpolateNormals(normals, date, nil)

	if result.TempAvg == nil {
		t.Fatal("expected non-nil TempAvg")
	}
	if math.Abs(*result.TempAvg-1.0) > 0.01 {
		t.Errorf("mid-January should be ~1.0, got %f", *result.TempAvg)
	}
}

func TestInterpolateNormals_BetweenMonths(t *testing.T) {
	// On Feb 1 (between Jan 15 and Feb 15), value should be between Jan and Feb
	normals := make([]ClimateNormal, 12)
	for i := range normals {
		v := float64((i + 1) * 10) // Jan=10, Feb=20, ...
		normals[i] = ClimateNormal{Month: i + 1, TempAvg: &v}
	}

	date := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	result := InterpolateNormals(normals, date, nil)

	if result.TempAvg == nil {
		t.Fatal("expected non-nil TempAvg")
	}
	if *result.TempAvg <= 10 || *result.TempAvg >= 20 {
		t.Errorf("Feb 1 should be between 10 and 20, got %f", *result.TempAvg)
	}
}

func TestInterpolateNormals_DecJanWrap(t *testing.T) {
	// Early January should interpolate between December and January
	normals := make([]ClimateNormal, 12)
	for i := range normals {
		normals[i] = ClimateNormal{Month: i + 1}
	}
	dec := -5.0
	jan := -3.0
	normals[11].TempAvg = &dec // December
	normals[0].TempAvg = &jan  // January

	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	result := InterpolateNormals(normals, date, nil)

	if result.TempAvg == nil {
		t.Fatal("expected non-nil TempAvg")
	}
	// Jan 1 is between Dec 15 and Jan 15, should be between -5 and -3
	if *result.TempAvg < -5 || *result.TempAvg > -3 {
		t.Errorf("Jan 1 should be between -5 and -3, got %f", *result.TempAvg)
	}
}

func TestInterpolateNormals_TempDiff(t *testing.T) {
	normals := make([]ClimateNormal, 12)
	jan := 0.0
	for i := range normals {
		normals[i] = ClimateNormal{Month: i + 1, TempAvg: &jan}
	}

	date := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	currentTemp := 5.0
	result := InterpolateNormals(normals, date, &currentTemp)

	if result.TempDiff == nil {
		t.Fatal("expected non-nil TempDiff")
	}
	if math.Abs(*result.TempDiff-5.0) > 0.01 {
		t.Errorf("temp diff should be 5.0, got %f", *result.TempDiff)
	}
}

func TestInterpolateNormals_PrecipMmDay(t *testing.T) {
	normals := make([]ClimateNormal, 12)
	precip := 31.0 // 31mm in January (31 days) = 1mm/day
	for i := range normals {
		normals[i] = ClimateNormal{Month: i + 1, PrecipMm: &precip}
	}

	date := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	result := InterpolateNormals(normals, date, nil)

	if result.PrecipMmDay == nil {
		t.Fatal("expected non-nil PrecipMmDay")
	}
	if math.Abs(*result.PrecipMmDay-1.0) > 0.01 {
		t.Errorf("precip should be ~1.0 mm/day, got %f", *result.PrecipMmDay)
	}
}
