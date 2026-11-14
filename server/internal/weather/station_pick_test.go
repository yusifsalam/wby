package weather

import (
	"slices"
	"testing"
	"time"
)

func TestPickStationPrefersCompleteNearbyStation(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	siilinkari := StationDistance{Station: Station{FMISID: 101311, Name: "Tampere Siilinkari"}, DistanceKM: 2.2}
	harmala := StationDistance{Station: Station{FMISID: 101124, Name: "Tampere Härmälä"}, DistanceKM: 3.7}
	pirkkala := StationDistance{Station: Station{FMISID: 101118, Name: "Pirkkala lentoasema"}, DistanceKM: 11}
	candidates := []StationDistance{siilinkari, harmala, pirkkala}
	obs := map[int]Observation{
		101311: {FMISID: 101311, ObservedAt: now.Add(-10 * time.Minute), Temperature: f(10.4), WindSpeed: f(1.1), WindDir: f(200), Humidity: f(98), Pressure: f(994.5)},
		101124: {FMISID: 101124, ObservedAt: now.Add(-10 * time.Minute), Temperature: f(10.5), Humidity: f(100), Pressure: f(994.5), Precip1h: f(0), WeatherCode: f(32), TotalCloudCover: f(8), Visibility: f(332)},
		101118: {FMISID: 101118, ObservedAt: now.Add(-10 * time.Minute), Temperature: f(8.6), WindSpeed: f(0.7), WindDir: f(180), Humidity: f(100), Pressure: f(994.9), WeatherCode: f(31), TotalCloudCover: f(9), Visibility: f(936)},
	}

	picked, merged, borrowed, ok := pickStation(candidates, obs, now)
	if !ok {
		t.Fatal("expected a station")
	}
	if picked.FMISID != harmala.FMISID {
		t.Fatalf("picked %s, want %s", picked.Name, harmala.Name)
	}
	if merged.Temperature == nil || *merged.Temperature != 10.5 {
		t.Errorf("Temperature = %v, want 10.5 from primary", merged.Temperature)
	}
	if merged.WindSpeed == nil || *merged.WindSpeed != 1.1 {
		t.Errorf("WindSpeed = %v, want 1.1 backfilled from Siilinkari", merged.WindSpeed)
	}
	if merged.WindDir == nil || *merged.WindDir != 200 {
		t.Errorf("WindDir = %v, want 200 backfilled from Siilinkari", merged.WindDir)
	}
	if merged.WeatherCode == nil || *merged.WeatherCode != 32 {
		t.Errorf("WeatherCode = %v, want 32 from primary", merged.WeatherCode)
	}
	if !merged.ObservedAt.Equal(obs[101124].ObservedAt) {
		t.Errorf("ObservedAt = %v, want primary's", merged.ObservedAt)
	}
	if obs[101124].WindSpeed != nil {
		t.Error("backfill mutated the source observation")
	}
	if len(borrowed) != 1 || borrowed[0].Station.FMISID != siilinkari.FMISID || borrowed[0].DistanceKM != siilinkari.DistanceKM {
		t.Fatalf("borrowed = %+v, want one entry from Siilinkari", borrowed)
	}
	if want := []string{"wind_speed", "wind_direction"}; !slices.Equal(borrowed[0].Fields, want) {
		t.Errorf("borrowed fields = %v, want %v", borrowed[0].Fields, want)
	}
}

func TestPickStationKeepsNearestWhenComplete(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	full := Observation{ObservedAt: now, Temperature: f(1), WindSpeed: f(1), Humidity: f(1), WeatherCode: f(1)}
	candidates := []StationDistance{
		{Station: Station{FMISID: 1}, DistanceKM: 1},
		{Station: Station{FMISID: 2}, DistanceKM: 2},
	}
	picked, _, borrowed, ok := pickStation(candidates, map[int]Observation{1: full, 2: full}, now)
	if !ok || picked.FMISID != 1 {
		t.Fatalf("picked %d, want 1", picked.FMISID)
	}
	if len(borrowed) != 0 {
		t.Errorf("borrowed = %+v, want none from a complete primary", borrowed)
	}
}

func TestPickStationSkipsStaleAndFallsBack(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	candidates := []StationDistance{
		{Station: Station{FMISID: 1}, DistanceKM: 1},
		{Station: Station{FMISID: 2}, DistanceKM: 5},
		{Station: Station{FMISID: 3}, DistanceKM: 9},
	}
	obs := map[int]Observation{
		1: {ObservedAt: now.Add(-3 * time.Hour), Temperature: f(20), WindSpeed: f(1), Humidity: f(1), WeatherCode: f(1)},
		3: {ObservedAt: now.Add(-5 * time.Minute), Temperature: f(5)},
	}
	picked, merged, _, ok := pickStation(candidates, obs, now)
	if !ok || picked.FMISID != 3 {
		t.Fatalf("picked %d, want 3 (only fresh)", picked.FMISID)
	}
	if merged.WindSpeed != nil {
		t.Error("stale station must not backfill")
	}

	stale := map[int]Observation{1: obs[1]}
	picked, merged, _, ok = pickStation(candidates, stale, now)
	if !ok || picked.FMISID != 1 || merged.Temperature == nil || *merged.Temperature != 20 {
		t.Fatalf("expected stale nearest fallback, got %d ok=%v", picked.FMISID, ok)
	}

	expired := map[int]Observation{1: {FMISID: 1, ObservedAt: now.Add(-4 * time.Hour), Temperature: f(20), WindSpeed: f(1)}}
	picked, merged, _, ok = pickStation(candidates, expired, now)
	if !ok || picked.FMISID != 1 {
		t.Fatalf("expected the nearest station even when its observation expired, got %d ok=%v", picked.FMISID, ok)
	}
	if merged.Temperature != nil || merged.WindSpeed != nil {
		t.Error("values older than observationExpiry must not be served")
	}
	if !merged.ObservedAt.Equal(now.Add(-4*time.Hour)) || merged.FMISID != 1 {
		t.Errorf("expired observation should keep its station and time, got %d at %v", merged.FMISID, merged.ObservedAt)
	}

	if _, _, _, ok := pickStation(candidates, nil, now); ok {
		t.Error("expected ok=false with no observations")
	}
}

func TestCurrentFeelsLike(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	now := time.Date(2026, 11, 14, 14, 20, 0, 0, time.UTC)
	hour := now.Truncate(time.Hour)
	hourly := []HourlyForecast{
		{Time: hour.Add(-time.Hour), FeelsLike: f(-1)},
		{Time: hour, FeelsLike: f(-5)},
		{Time: hour.Add(time.Hour), FeelsLike: f(-6)},
	}

	withWind := Observation{Temperature: f(0), WindSpeed: f(6), Humidity: f(80)}
	if got, want := currentFeelsLike(withWind, hourly, now), withWind.FeelsLike(); got == nil || *got != *want {
		t.Errorf("with observed wind = %v, want the observation's %v", got, *want)
	}

	noWind := Observation{Temperature: f(0), Humidity: f(80)}
	if got := currentFeelsLike(noWind, hourly, now); got == nil || *got != -5 {
		t.Errorf("without wind = %v, want -5 from the current forecast hour", got)
	}
	if got := currentFeelsLike(noWind, hourly[2:], now); got != nil {
		t.Errorf("without wind or a forecast for this hour = %v, want nil", *got)
	}
	if got := currentFeelsLike(noWind, nil, now); got != nil {
		t.Errorf("without wind or any forecast = %v, want nil", *got)
	}
	if got := currentFeelsLike(Observation{WindSpeed: f(3)}, hourly, now); got != nil {
		t.Errorf("without a temperature = %v, want nil", *got)
	}
}
