package fmi

import (
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
}
