package fmi

import (
	"math"
	"os"
	"testing"
)

func TestParseObservations(t *testing.T) {
	data, err := os.ReadFile("testdata/observations.xml")
	if err != nil {
		t.Fatal(err)
	}

	result, err := ParseObservations(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Stations) == 0 {
		t.Fatal("expected at least one station")
	}

	station := result.Stations[0]
	if station.Name == "" {
		t.Error("station name should not be empty")
	}
	if station.Lat == 0 || station.Lon == 0 {
		t.Errorf("station coordinates should not be zero: %f, %f", station.Lat, station.Lon)
	}

	if len(result.Observations) == 0 {
		t.Fatal("expected at least one observation")
	}

	// Observations are sorted by time, so last is latest
	obs := result.Observations[len(result.Observations)-1]
	if obs.Temperature == nil {
		t.Error("latest observation should have temperature")
	}
}

func TestCircularMeanDegreesPtrWrapAround(t *testing.T) {
	got := circularMeanDegreesPtr([]float64{350, 10})
	if got == nil {
		t.Fatal("expected mean direction")
	}
	if math.Abs(*got-0) > 1e-6 && math.Abs(*got-360) > 1e-6 {
		t.Fatalf("expected mean direction near north, got %f", *got)
	}
}

func TestCircularMeanDegreesPtrEmpty(t *testing.T) {
	if got := circularMeanDegreesPtr(nil); got != nil {
		t.Fatalf("expected nil for empty input, got %v", *got)
	}
}

func TestParseObservationsStationNamePreference(t *testing.T) {
	data, err := os.ReadFile("testdata/observations.xml")
	if err != nil {
		t.Fatal(err)
	}

	result, err := ParseObservations(data)
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, st := range result.Stations {
		if st.FMISID != 100971 {
			continue
		}
		found = true
		if st.Name != "Helsinki Kaisaniemi" {
			t.Fatalf("unexpected station name for FMISID 100971: %q", st.Name)
		}
		if st.WMOCode != "2978" {
			t.Fatalf("unexpected WMO code for FMISID 100971: %q", st.WMOCode)
		}
	}
	if !found {
		t.Fatal("expected station FMISID 100971 in fixture")
	}
}

func TestParseForecast(t *testing.T) {
	data, err := os.ReadFile("testdata/forecast.xml")
	if err != nil {
		t.Fatal(err)
	}

	result, err := ParseForecast(data, 60.17, 24.94)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) == 0 {
		t.Fatal("expected at least one daily forecast")
	}

	day := result[0]
	if day.TempHigh == nil {
		t.Error("expected temp_high to be set")
	}
	if day.TempLow == nil {
		t.Error("expected temp_low to be set")
	}
	if day.Date.IsZero() {
		t.Error("expected date to be set")
	}
	if day.WindSpeed == nil {
		t.Error("expected wind_speed_avg to be set")
	}
	if day.WindDir == nil {
		t.Error("expected wind_direction_avg to be set")
	}
	if day.HumidityAvg == nil {
		t.Error("expected humidity_avg to be set")
	}
}

func TestParseHourlyForecast(t *testing.T) {
	data, err := os.ReadFile("testdata/forecast.xml")
	if err != nil {
		t.Fatal(err)
	}

	result, err := ParseHourlyForecast(data, 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) == 0 {
		t.Fatal("expected hourly forecast entries")
	}
	if len(result) > 12 {
		t.Fatalf("expected at most 12 entries, got %d", len(result))
	}
	if result[0].Time.IsZero() {
		t.Fatal("expected first hourly time to be set")
	}
	if result[0].WindSpeed == nil {
		t.Error("expected hourly wind_speed to be set")
	}
	if result[0].WindDir == nil {
		t.Error("expected hourly wind_direction to be set")
	}
	if result[0].Humidity == nil {
		t.Error("expected hourly humidity to be set")
	}
	for i := 1; i < len(result); i++ {
		if result[i].Time.Before(result[i-1].Time) {
			t.Fatalf("hourly forecast not sorted: %s before %s", result[i].Time, result[i-1].Time)
		}
	}
}

func TestParseForecastDefaultParamsFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/forecast.xml")
	if err != nil {
		t.Fatal(err)
	}

	result, err := ParseForecast(data, 60.17, 24.94)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) == 0 {
		t.Fatal("expected daily forecast entries")
	}

	day := result[0]
	if day.TempAvg == nil {
		t.Error("expected temperature_avg")
	}
	if day.PressureAvg == nil {
		t.Error("expected pressure_avg")
	}
	if day.DewPointAvg == nil {
		t.Error("expected dew_point_avg")
	}
	if day.TotalCloudCoverAvg == nil {
		t.Error("expected total_cloud_cover_avg")
	}
	if day.HourlyMaximumGustMax == nil {
		t.Error("expected hourly_maximum_gust_max")
	}
	if day.ProbabilityThunderstormAvg == nil {
		t.Error("expected probability_thunderstorm_avg")
	}
	if day.PotentialPrecipitationTypeMode == nil {
		t.Error("expected potential_precipitation_type_mode")
	}
	if day.PrecipitationTypeMode == nil {
		t.Error("expected precipitation_type_mode")
	}
	// RadiationGlobalAvg is omitted: the fixture was captured at night so all
	// radiation values are NaN, which the parser correctly maps to nil.
	if day.WeatherNumberMode == nil {
		t.Error("expected weather_number_mode")
	}
	if day.WindUMSAvg == nil {
		t.Error("expected wind_ums_avg")
	}
	if day.WindVMSAvg == nil {
		t.Error("expected wind_vms_avg")
	}
	if day.WindVectorMSAvg == nil {
		t.Error("expected wind_vector_ms_avg")
	}
}
