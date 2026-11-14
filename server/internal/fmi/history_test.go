package fmi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"wby/internal/weather"
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

func TestParseHourlyObservationsFromInstantQuery(t *testing.T) {
	data, err := os.ReadFile("testdata/instant_hourly_observations.xml")
	if err != nil {
		t.Fatal(err)
	}
	recs, err := ParseHourlyObservations(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) < 168 {
		t.Fatalf("got %d records, want a week of hours", len(recs))
	}
	withAll := 0
	for _, r := range recs {
		if r.Time.Minute() != 0 {
			t.Errorf("record at %v is not on the hour", r.Time)
		}
		if r.WindGust != nil || r.PrecipMm != nil {
			t.Errorf("record at %v carries gust/precip the query never asked for", r.Time)
		}
		if r.Temp != nil && r.Humidity != nil && r.WindSpeed != nil {
			withAll++
		}
	}
	if withAll < 150 {
		t.Errorf("only %d of %d records have temperature, humidity and wind", withAll, len(recs))
	}
}

func TestFetchHourlyPrecipitationNearDropsRecordsRepeatedAcrossChunks(t *testing.T) {
	fixture, err := os.ReadFile("testdata/hourly_observations.xml")
	if err != nil {
		t.Fatal(err)
	}
	body := bytes.ReplaceAll(fixture, []byte("param=TA_PT1H_AVG"), []byte("param=PRA_PT1H_ACC"))
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write(body)
	}))
	defer srv.Close()

	start := time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2020, 3, 10, 0, 0, 0, 0, time.UTC)
	got, err := NewClient(srv.URL, "", "").FetchHourlyPrecipitationNear(context.Background(), 60.17, 24.94, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if n := requests.Load(); n < 2 {
		t.Fatalf("got %d requests, want the range split into at least 2 chunks", n)
	}
	if got.Station.FMISID != 100971 {
		t.Fatalf("station = %d, want 100971", got.Station.FMISID)
	}
	if len(got.Hourly) != 6 {
		t.Fatalf("got %d records, want 6 (each hour once)", len(got.Hourly))
	}
	for i := 1; i < len(got.Hourly); i++ {
		if !got.Hourly[i].Time.After(got.Hourly[i-1].Time) {
			t.Fatalf("records not strictly ascending at %d: %v then %v", i, got.Hourly[i-1].Time, got.Hourly[i].Time)
		}
	}
}

func TestPickPrecipitationGauge(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	gauge := func(fmisid int, distKm float64, hours ...int) *weather.PrecipitationObservations {
		g := &weather.PrecipitationObservations{Station: weather.Station{FMISID: fmisid}, DistanceKM: distKm}
		zero := 0.0
		for _, h := range hours {
			g.Hourly = append(g.Hourly, weather.HourlyRecord{Time: start.Add(time.Duration(h) * time.Hour), PrecipMm: &zero})
		}
		return g
	}
	hours := func(from, to int) []int {
		var out []int
		for h := from; h <= to; h++ {
			out = append(out, h)
		}
		return out
	}

	t.Run("complete gauge beats a nearer one with gaps", func(t *testing.T) {
		near := gauge(1, 2, hours(1, 10)...)
		far := gauge(2, 20, hours(0, 100)...)
		if got := pickPrecipitationGauge([]*weather.PrecipitationObservations{near, far}, start); got != far {
			t.Fatalf("picked gauge %d, want 2", got.Station.FMISID)
		}
	})
	t.Run("nearest of the complete gauges", func(t *testing.T) {
		near := gauge(1, 2, hours(1, 95)...)
		far := gauge(2, 20, hours(1, 100)...)
		if got := pickPrecipitationGauge([]*weather.PrecipitationObservations{far, near}, start); got != near {
			t.Fatalf("picked gauge %d, want 1 (95 of 100 hours is enough)", got.Station.FMISID)
		}
	})
	t.Run("most hours when none is complete", func(t *testing.T) {
		near := gauge(1, 2, hours(1, 10)...)
		mid := gauge(2, 10, hours(1, 40)...)
		late := gauge(3, 30, 100)
		if got := pickPrecipitationGauge([]*weather.PrecipitationObservations{near, mid, late}, start); got != mid {
			t.Fatalf("picked gauge %d, want 2", got.Station.FMISID)
		}
	})
	t.Run("no gauges", func(t *testing.T) {
		if got := pickPrecipitationGauge(nil, start); got != nil {
			t.Fatalf("picked %v, want nil", got)
		}
	})
}
