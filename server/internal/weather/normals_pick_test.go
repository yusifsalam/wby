package weather

import "testing"

func TestPickNormalsStationsPrefersLongerRecord(t *testing.T) {
	harmala := DailyNormalsCandidate{StationDistance: StationDistance{Station: Station{FMISID: 101124, Name: "Härmälä"}, DistanceKM: 3.7}, DailyYears: 23.6, HourlyYears: 13.8, HasTemp: true, HasHourly: true}
	pirkkala := DailyNormalsCandidate{StationDistance: StationDistance{Station: Station{FMISID: 101118, Name: "Pirkkala"}, DistanceKM: 11.5}, DailyYears: 30, HourlyYears: 1.1, HasTemp: true, HasHourly: false}

	primary, hourly := pickNormalsStations([]DailyNormalsCandidate{harmala, pirkkala}, 30)
	if primary == nil || primary.FMISID != pirkkala.FMISID {
		t.Fatalf("primary = %v, want Pirkkala", primary)
	}
	if hourly == nil || hourly.FMISID != harmala.FMISID {
		t.Fatalf("hourly = %v, want Härmälä", hourly)
	}
}

func TestPickNormalsStationsKeepsNearestWithinAYear(t *testing.T) {
	near := DailyNormalsCandidate{StationDistance: StationDistance{Station: Station{FMISID: 1}, DistanceKM: 1}, DailyYears: 29.2, HourlyYears: 20, HasTemp: true, HasHourly: true}
	far := DailyNormalsCandidate{StationDistance: StationDistance{Station: Station{FMISID: 2}, DistanceKM: 6}, DailyYears: 30, HourlyYears: 30, HasTemp: true, HasHourly: true}

	primary, hourly := pickNormalsStations([]DailyNormalsCandidate{near, far}, 30)
	if primary == nil || primary.FMISID != 1 {
		t.Fatalf("primary = %v, want nearest", primary)
	}
	if hourly != nil {
		t.Errorf("hourly = %v, want nil when the primary has curves", hourly)
	}
}

func TestPickNormalsStationsWithoutTemperature(t *testing.T) {
	only := DailyNormalsCandidate{StationDistance: StationDistance{Station: Station{FMISID: 1}, DistanceKM: 1}, HasHourly: true}
	if primary, _ := pickNormalsStations([]DailyNormalsCandidate{only}, 30); primary != nil {
		t.Errorf("primary = %v, want nil", primary)
	}
	if primary, _ := pickNormalsStations(nil, 30); primary != nil {
		t.Errorf("primary = %v, want nil for no candidates", primary)
	}
}

func TestMergeHourlyNormals(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	dst := []DailyClimateNormal{
		{Month: 9, Day: 5, TempAvg: f(12), PrecipMm: f(2), DailyYears: 30},
		{Month: 9, Day: 6, TempAvg: f(11), DailyYears: 30},
	}
	curve := make([]float64, 24)
	for h := range curve {
		curve[h] = 10 + float64(h%12)
	}
	src := []DailyClimateNormal{
		{Month: 9, Day: 5, TempAvg: f(99), PrecipMm: f(99), WindAvg: f(4), FeelsLikeAvg: f(10), TempHourly: curve, TempHourlyP90: shiftedCurve(curve, 3), HourlyYears: 14},
	}
	mergeHourlyNormals(dst, src)

	got := dst[0]
	if *got.TempAvg != 12 || *got.PrecipMm != 2 || got.DailyYears != 30 {
		t.Errorf("daily fields changed: %+v", got)
	}
	if got.WindAvg == nil || *got.WindAvg != 4 || len(got.TempHourly) != 24 || got.HourlyYears != 14 {
		t.Errorf("hourly fields not merged: %+v", got)
	}
	// src's curve averages 15.5; shifting onto dst's 12 moves everything by -3.5.
	if m := mean(got.TempHourly); m < 11.99 || m > 12.01 {
		t.Errorf("merged curve mean = %.2f, want 12", m)
	}
	if got.TempHourly[1] != 7.5 || got.TempHourlyP90[1] != 10.5 {
		t.Errorf("curve shift wrong: temp[1]=%v p90[1]=%v", got.TempHourly[1], got.TempHourlyP90[1])
	}
	if got.FeelsLikeAvg == nil || *got.FeelsLikeAvg != 6.5 {
		t.Errorf("FeelsLikeAvg = %v, want 6.5", got.FeelsLikeAvg)
	}
	if got.TempHourlyP10 != nil {
		t.Errorf("nil curve became %v", got.TempHourlyP10)
	}
	if dst[1].WindAvg != nil || dst[1].TempHourly != nil {
		t.Errorf("day without source changed: %+v", dst[1])
	}
}
