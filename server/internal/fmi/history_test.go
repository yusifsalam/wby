package fmi

import (
	"os"
	"testing"
	"time"
)

func TestParseDailyObservations(t *testing.T) {
	data, err := os.ReadFile("testdata/daily_observations.xml")
	if err != nil {
		t.Fatal(err)
	}
	recs, err := ParseDailyObservations(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 4 {
		t.Fatalf("got %d records, want 4 (27 Feb .. 1 Mar 2020)", len(recs))
	}
	first := recs[0]
	if !first.Date.Equal(time.Date(2020, 2, 27, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("first date = %v", first.Date)
	}
	if first.TempAvg == nil || first.TempHigh == nil || first.TempLow == nil || first.PrecipMm == nil {
		t.Fatalf("first record has nil fields: %+v", first)
	}
	if *first.TempLow > *first.TempAvg || *first.TempAvg > *first.TempHigh {
		t.Errorf("min/avg/max out of order: %v %v %v", *first.TempLow, *first.TempAvg, *first.TempHigh)
	}
	if got := recs[2].Date; !got.Equal(time.Date(2020, 2, 29, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("third date = %v, want 29 Feb", got)
	}
}

func TestParseHourlyObservations(t *testing.T) {
	data, err := os.ReadFile("testdata/hourly_observations.xml")
	if err != nil {
		t.Fatal(err)
	}
	recs, err := ParseHourlyObservations(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 6 {
		t.Fatalf("got %d records, want 6 (00..05 UTC)", len(recs))
	}
	for i, r := range recs {
		want := time.Date(2020, 2, 29, i, 0, 0, 0, time.UTC)
		if !r.Time.Equal(want) {
			t.Errorf("record %d time = %v, want %v", i, r.Time, want)
		}
		if r.Temp == nil || *r.Temp < -40 || *r.Temp > 40 {
			t.Errorf("record %d temp %v out of range", i, r.Temp)
		}
		if r.Humidity == nil || *r.Humidity < 0 || *r.Humidity > 100 {
			t.Errorf("record %d humidity %v out of range", i, r.Humidity)
		}
		if r.WindSpeed == nil || r.WindGust == nil || *r.WindGust < *r.WindSpeed {
			t.Errorf("record %d wind %v gust %v: want both present, gust >= speed", i, r.WindSpeed, r.WindGust)
		}
	}
}
