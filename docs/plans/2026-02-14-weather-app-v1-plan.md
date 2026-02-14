# Weather App v1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Finland weather app (iOS 26) backed by a Go caching server that serves FMI Open Data without clients ever hitting FMI directly.

**Architecture:** Go server fetches FMI WFS data (observations on schedule, forecasts on demand), stores in PostgreSQL/PostGIS, and serves a JSON REST API. SwiftUI iOS app consumes this single endpoint. Everything runs on one VPS via Docker Compose with Caddy for HTTPS.

**Tech Stack:** Go (stdlib + pgx), PostgreSQL 18 + PostGIS, Docker Compose, Caddy, SwiftUI (iOS 26), CoreLocation

**Design doc:** `docs/plans/2026-02-14-weather-app-v1-design.md`

---

## Task 1: Go Project Scaffold

**Files:**
- Create: `server/go.mod`
- Create: `server/cmd/server/main.go`
- Create: `server/internal/config/config.go`
- Create: `server/.env.example`

**Step 1: Initialize Go module**

```bash
cd server
go mod init github.com/salami-weather/server
```

**Step 2: Create entry point**

```go
// server/cmd/server/main.go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/salami-weather/server/internal/config"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	slog.Info("server stopped")
}
```

**Step 3: Create config loader**

```go
// server/internal/config/config.go
package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	FMIBaseURL  string
}

func Load() Config {
	return Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://weather:weather@localhost:5432/weather?sslmode=disable"),
		FMIBaseURL:  getEnv("FMI_BASE_URL", "https://opendata.fmi.fi/wfs"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

**Step 4: Create .env.example**

```
PORT=8080
DATABASE_URL=postgres://weather:weather@localhost:5432/weather?sslmode=disable
FMI_BASE_URL=https://opendata.fmi.fi/wfs
```

**Step 5: Verify it compiles**

Run: `cd server && go build ./cmd/server`
Expected: No errors, binary created.

**Step 6: Commit**

```bash
git add server/
git commit -m "feat: scaffold Go server with health endpoint and config"
```

---

## Task 2: Docker Compose + PostgreSQL/PostGIS

**Files:**
- Create: `server/docker-compose.yml`
- Create: `server/Dockerfile`
- Create: `server/migrations/001_initial.sql`

**Step 1: Create Docker Compose file**

```yaml
# server/docker-compose.yml
services:
  db:
    image: postgis/postgis:18-3.5
    environment:
      POSTGRES_USER: weather
      POSTGRES_PASSWORD: weather
      POSTGRES_DB: weather
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U weather"]
      interval: 5s
      timeout: 3s
      retries: 5

  server:
    build: .
    ports:
      - "8080:8080"
    environment:
      PORT: "8080"
      DATABASE_URL: "postgres://weather:weather@db:5432/weather?sslmode=disable"
      FMI_BASE_URL: "https://opendata.fmi.fi/wfs"
    depends_on:
      db:
        condition: service_healthy

volumes:
  pgdata:
```

**Step 2: Create initial migration**

```sql
-- server/migrations/001_initial.sql
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE stations (
    fmisid    INTEGER PRIMARY KEY,
    name      TEXT NOT NULL,
    geom      GEOGRAPHY(POINT, 4326) NOT NULL,
    wmo_code  TEXT
);

CREATE INDEX idx_stations_geom ON stations USING GIST (geom);

CREATE TABLE observations (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    fmisid      INTEGER NOT NULL REFERENCES stations(fmisid),
    observed_at TIMESTAMPTZ NOT NULL,
    temperature DOUBLE PRECISION,
    wind_speed  DOUBLE PRECISION,
    wind_dir    DOUBLE PRECISION,
    humidity    DOUBLE PRECISION,
    pressure    DOUBLE PRECISION,
    UNIQUE (fmisid, observed_at)
);

CREATE INDEX idx_observations_fmisid_time ON observations (fmisid, observed_at DESC);

CREATE TABLE forecasts (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    grid_lat     DOUBLE PRECISION NOT NULL,
    grid_lon     DOUBLE PRECISION NOT NULL,
    forecast_for DATE NOT NULL,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    temp_high    DOUBLE PRECISION,
    temp_low     DOUBLE PRECISION,
    wind_speed   DOUBLE PRECISION,
    precip_mm    DOUBLE PRECISION,
    symbol       TEXT,
    UNIQUE (grid_lat, grid_lon, forecast_for)
);

CREATE INDEX idx_forecasts_grid_date ON forecasts (grid_lat, grid_lon, forecast_for);
```

**Step 3: Create Dockerfile**

```dockerfile
# server/Dockerfile
FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /server /server
EXPOSE 8080
CMD ["/server"]
```

**Step 4: Start database and verify**

Run: `cd server && docker compose up db -d && sleep 3 && docker compose exec db psql -U weather -c "SELECT PostGIS_Version();"`
Expected: PostGIS version string printed.

**Step 5: Verify tables exist**

Run: `docker compose exec db psql -U weather -c "\dt"`
Expected: stations, observations, forecasts tables listed.

**Step 6: Commit**

```bash
git add server/docker-compose.yml server/Dockerfile server/migrations/
git commit -m "feat: add Docker Compose with PostGIS and initial schema"
```

---

## Task 3: Domain Models

**Files:**
- Create: `server/internal/weather/models.go`

**Step 1: Define domain types**

```go
// server/internal/weather/models.go
package weather

import "time"

type Station struct {
	FMISID  int
	Name    string
	Lat     float64
	Lon     float64
	WMOCode string
}

type Observation struct {
	FMISID      int
	ObservedAt  time.Time
	Temperature *float64
	WindSpeed   *float64
	WindDir     *float64
	Humidity    *float64
	Pressure    *float64
}

type DailyForecast struct {
	GridLat   float64
	GridLon   float64
	Date      time.Time
	FetchedAt time.Time
	TempHigh  *float64
	TempLow   *float64
	WindSpeed *float64
	PrecipMM  *float64
	Symbol    *string
}

type CurrentWeather struct {
	Station     Station
	DistanceKM  float64
	Observation Observation
}

type WeatherResponse struct {
	Current  CurrentWeather
	Forecast []DailyForecast
}
```

All numeric fields are pointers because FMI can return `NaN` for missing data.

**Step 2: Commit**

```bash
git add server/internal/weather/
git commit -m "feat: define weather domain models"
```

---

## Task 4: FMI WFS XML Parser

This is the trickiest part. FMI returns WFS XML where each parameter is a separate `wfs:member` containing `wml2:MeasurementTimeseries` with time-value pairs.

**Files:**
- Create: `server/internal/fmi/parser.go`
- Create: `server/internal/fmi/parser_test.go`
- Create: `server/internal/fmi/testdata/observations.xml`
- Create: `server/internal/fmi/testdata/forecast.xml`

**Step 1: Save test fixture — observations XML**

Fetch a real response and save it:

```bash
curl -s "https://opendata.fmi.fi/wfs?service=WFS&version=2.0.0&request=getFeature&storedquery_id=fmi::observations::weather::timevaluepair&place=helsinki&parameters=temperature,windspeedms,winddirection,humidity,pressure&timestep=60&maxlocations=1" > server/internal/fmi/testdata/observations.xml
```

**Step 2: Save test fixture — forecast XML**

```bash
curl -s "https://opendata.fmi.fi/wfs?service=WFS&version=2.0.0&request=getFeature&storedquery_id=fmi::forecast::harmonie::surface::point::timevaluepair&place=helsinki&parameters=temperature,windspeedms,winddirection,humidity,precipitation1h,weathersymbol3&timestep=60&maxlocations=1" > server/internal/fmi/testdata/forecast.xml
```

**Step 3: Write the failing test for observation parsing**

```go
// server/internal/fmi/parser_test.go
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

	obs := result.Observations[len(result.Observations)-1] // latest
	if obs.Temperature == nil {
		t.Error("latest observation should have temperature")
	}
}
```

**Step 4: Run test to verify it fails**

Run: `cd server && go test ./internal/fmi/ -v -run TestParseObservations`
Expected: FAIL — `ParseObservations` not defined.

**Step 5: Implement observation parser**

```go
// server/internal/fmi/parser.go
package fmi

import (
	"encoding/xml"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/salami-weather/server/internal/weather"
)

// WFS XML types — shared across observations and forecasts.
// Go's encoding/xml matches on local element name; we ignore namespace prefixes.

type featureCollection struct {
	XMLName xml.Name `xml:"FeatureCollection"`
	Members []member `xml:"member"`
}

type member struct {
	Observation pointTimeSeries `xml:"PointTimeSeriesObservation"`
}

type pointTimeSeries struct {
	ObservedProperty observedProperty `xml:"observedProperty"`
	FeatureOfInterest featureOfInterest `xml:"featureOfInterest"`
	Result           tsResult          `xml:"result"`
}

type observedProperty struct {
	Href string `xml:"href,attr"`
}

type featureOfInterest struct {
	Feature spatialFeature `xml:"SF_SpatialSamplingFeature"`
}

type spatialFeature struct {
	SampledFeature sampledFeature `xml:"sampledFeature"`
	Shape          shape          `xml:"shape"`
}

type sampledFeature struct {
	LocationCollection locationCollection `xml:"LocationCollection"`
}

type locationCollection struct {
	Members []locationMember `xml:"member"`
}

type locationMember struct {
	Location location `xml:"Location"`
}

type location struct {
	Identifier string `xml:"identifier"`
	Names      []gmlName `xml:"name"`
}

type gmlName struct {
	CodeSpace string `xml:"codeSpace,attr"`
	Value     string `xml:",chardata"`
}

type shape struct {
	Point      gmlPoint `xml:"Point"`
	MultiPoint multiPoint `xml:"MultiPoint"`
}

type gmlPoint struct {
	Pos string `xml:"pos"`
}

type multiPoint struct {
	Points []gmlPoint `xml:"pointMember>Point"`
}

type tsResult struct {
	TimeSeries measurementTimeSeries `xml:"MeasurementTimeseries"`
}

type measurementTimeSeries struct {
	Points []measurementPoint `xml:"point"`
}

type measurementPoint struct {
	TVP timeValuePair `xml:"MeasurementTVP"`
}

type timeValuePair struct {
	Time  string `xml:"time"`
	Value string `xml:"value"`
}

// ObservationResult holds parsed observation data from FMI.
type ObservationResult struct {
	Stations     []weather.Station
	Observations []weather.Observation
}

// ParseObservations parses an FMI WFS observation response.
func ParseObservations(data []byte) (*ObservationResult, error) {
	var fc featureCollection
	if err := xml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("unmarshal WFS: %w", err)
	}

	if len(fc.Members) == 0 {
		return &ObservationResult{}, nil
	}

	// Each member is one parameter's time series for one station.
	// Group time-value pairs by timestamp across all parameters.
	stationMap := make(map[int]*weather.Station)
	type obsKey struct {
		fmisid int
		t      time.Time
	}
	obsMap := make(map[obsKey]*weather.Observation)

	for _, m := range fc.Members {
		param := extractParam(m.Observation.ObservedProperty.Href)
		fmisid, name, lat, lon, wmo := extractStationInfo(m.Observation)

		if _, ok := stationMap[fmisid]; !ok {
			stationMap[fmisid] = &weather.Station{
				FMISID:  fmisid,
				Name:    name,
				Lat:     lat,
				Lon:     lon,
				WMOCode: wmo,
			}
		}

		for _, pt := range m.Observation.Result.TimeSeries.Points {
			t, err := time.Parse(time.RFC3339, pt.TVP.Time)
			if err != nil {
				continue
			}
			val := parseFloat(pt.TVP.Value)

			key := obsKey{fmisid: fmisid, t: t}
			obs, ok := obsMap[key]
			if !ok {
				obs = &weather.Observation{FMISID: fmisid, ObservedAt: t}
				obsMap[key] = obs
			}

			switch param {
			case "temperature":
				obs.Temperature = val
			case "windspeedms":
				obs.WindSpeed = val
			case "winddirection":
				obs.WindDir = val
			case "humidity":
				obs.Humidity = val
			case "pressure":
				obs.Pressure = val
			}
		}
	}

	result := &ObservationResult{}
	for _, s := range stationMap {
		result.Stations = append(result.Stations, *s)
	}
	for _, o := range obsMap {
		result.Observations = append(result.Observations, *o)
	}

	// Sort for deterministic output: stations by FMISID, observations by time.
	slices.SortFunc(result.Stations, func(a, b weather.Station) int {
		return a.FMISID - b.FMISID
	})
	slices.SortFunc(result.Observations, func(a, b weather.Observation) int {
		return a.ObservedAt.Compare(b.ObservedAt)
	})

	return result, nil
}

// extractParam gets the parameter name from an observedProperty href.
// e.g., "...&param=temperature&..." → "temperature"
func extractParam(href string) string {
	for _, part := range strings.Split(href, "&") {
		if strings.HasPrefix(part, "param=") {
			return strings.TrimPrefix(part, "param=")
		}
	}
	// Fallback: last path segment might contain it
	parts := strings.Split(href, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func extractStationInfo(pts pointTimeSeries) (fmisid int, name string, lat, lon float64, wmo string) {
	foi := pts.FeatureOfInterest.Feature
	for _, lm := range foi.SampledFeature.LocationCollection.Members {
		loc := lm.Location
		fmisid, _ = strconv.Atoi(loc.Identifier)
		for _, n := range loc.Names {
			switch {
			case strings.Contains(n.CodeSpace, "name"):
				name = n.Value
			case strings.Contains(n.CodeSpace, "wmo"):
				wmo = n.Value
			}
		}
	}

	pos := foi.Shape.Point.Pos
	if pos == "" && len(foi.Shape.MultiPoint.Points) > 0 {
		pos = foi.Shape.MultiPoint.Points[0].Pos
	}
	lat, lon = parsePos(pos)
	return
}

func parsePos(pos string) (float64, float64) {
	parts := strings.Fields(pos)
	if len(parts) != 2 {
		return 0, 0
	}
	lat, _ := strconv.ParseFloat(parts[0], 64)
	lon, _ := strconv.ParseFloat(parts[1], 64)
	return lat, lon
}

func parseFloat(s string) *float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) {
		return nil
	}
	return &v
}
```

**Step 6: Run test to verify it passes**

Run: `cd server && go test ./internal/fmi/ -v -run TestParseObservations`
Expected: PASS

**Step 7: Write the failing test for forecast parsing**

Add to `parser_test.go`:

```go
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
```

**Step 8: Run test to verify it fails**

Run: `cd server && go test ./internal/fmi/ -v -run TestParseForecast`
Expected: FAIL — `ParseForecast` not defined.

**Step 9: Implement forecast parser**

Add to `parser.go`:

```go
// ParseForecast parses an FMI WFS Harmonie forecast response and aggregates
// hourly values into daily forecasts (high, low, avg wind, total precip).
func ParseForecast(data []byte, gridLat, gridLon float64) ([]weather.DailyForecast, error) {
	var fc featureCollection
	if err := xml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("unmarshal WFS forecast: %w", err)
	}

	// Collect hourly values per parameter.
	type hourlyEntry struct {
		t   time.Time
		val float64
	}
	params := make(map[string][]hourlyEntry)

	for _, m := range fc.Members {
		param := extractParam(m.Observation.ObservedProperty.Href)
		for _, pt := range m.Observation.Result.TimeSeries.Points {
			t, err := time.Parse(time.RFC3339, pt.TVP.Time)
			if err != nil {
				continue
			}
			val := parseFloat(pt.TVP.Value)
			if val == nil {
				continue
			}
			params[param] = append(params[param], hourlyEntry{t: t, val: *val})
		}
	}

	// Aggregate into daily buckets.
	type dayBucket struct {
		temps   []float64
		winds   []float64
		precip  float64
		symbols []string
	}
	days := make(map[string]*dayBucket)
	dayOrder := []string{}

	addDay := func(dateKey string) *dayBucket {
		if _, ok := days[dateKey]; !ok {
			days[dateKey] = &dayBucket{}
			dayOrder = append(dayOrder, dateKey)
		}
		return days[dateKey]
	}

	for _, e := range params["temperature"] {
		dk := e.t.Format("2006-01-02")
		b := addDay(dk)
		b.temps = append(b.temps, e.val)
	}
	for _, e := range params["windspeedms"] {
		dk := e.t.Format("2006-01-02")
		b := addDay(dk)
		b.winds = append(b.winds, e.val)
	}
	for _, e := range params["precipitation1h"] {
		dk := e.t.Format("2006-01-02")
		b := addDay(dk)
		b.precip += e.val
	}
	for _, e := range params["weathersymbol3"] {
		dk := e.t.Format("2006-01-02")
		b := addDay(dk)
		b.symbols = append(b.symbols, strconv.Itoa(int(e.val)))
	}

	now := time.Now()
	var forecasts []weather.DailyForecast
	for _, dk := range dayOrder {
		b := days[dk]
		date, _ := time.Parse("2006-01-02", dk)
		f := weather.DailyForecast{
			GridLat:   gridLat,
			GridLon:   gridLon,
			Date:      date,
			FetchedAt: now,
		}
		if len(b.temps) > 0 {
			hi, lo := b.temps[0], b.temps[0]
			for _, t := range b.temps[1:] {
				if t > hi { hi = t }
				if t < lo { lo = t }
			}
			f.TempHigh = &hi
			f.TempLow = &lo
		}
		if len(b.winds) > 0 {
			avg := 0.0
			for _, w := range b.winds { avg += w }
			avg /= float64(len(b.winds))
			f.WindSpeed = &avg
		}
		precip := b.precip
		f.PrecipMM = &precip
		if len(b.symbols) > 0 {
			// Use the most common symbol (mode) as the day's symbol.
			f.Symbol = &b.symbols[len(b.symbols)/2] // midday symbol as representative
		}
		forecasts = append(forecasts, f)
	}
	return forecasts, nil
}
```

**Step 10: Run all tests**

Run: `cd server && go test ./internal/fmi/ -v`
Expected: Both tests PASS.

**Step 11: Commit**

```bash
git add server/internal/fmi/
git commit -m "feat: FMI WFS XML parser for observations and forecasts"
```

---

## Task 5: FMI HTTP Client

**Files:**
- Create: `server/internal/fmi/client.go`

**Step 1: Implement FMI HTTP client**

```go
// server/internal/fmi/client.go
package fmi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/salami-weather/server/internal/weather"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchObservations retrieves current weather observations for all Finnish stations.
func (c *Client) FetchObservations(ctx context.Context) (*ObservationResult, error) {
	params := url.Values{
		"service":         {"WFS"},
		"version":         {"2.0.0"},
		"request":         {"getFeature"},
		"storedquery_id":  {"fmi::observations::weather::timevaluepair"},
		"parameters":      {"temperature,windspeedms,winddirection,humidity,pressure"},
		"timestep":        {"10"},
		"maxlocations":    {"200"},
	}

	data, err := c.fetch(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch observations: %w", err)
	}
	return ParseObservations(data)
}

// FetchForecast retrieves Harmonie point forecast for a given coordinate.
func (c *Client) FetchForecast(ctx context.Context, lat, lon float64) ([]weather.DailyForecast, error) {
	params := url.Values{
		"service":        {"WFS"},
		"version":        {"2.0.0"},
		"request":        {"getFeature"},
		"storedquery_id": {"fmi::forecast::harmonie::surface::point::timevaluepair"},
		"latlon":         {fmt.Sprintf("%f,%f", lat, lon)},
		"parameters":     {"temperature,windspeedms,winddirection,humidity,precipitation1h,weathersymbol3"},
		"timestep":       {"60"},
	}

	data, err := c.fetch(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch forecast: %w", err)
	}
	return ParseForecast(data, lat, lon)
}

func (c *Client) fetch(ctx context.Context, params url.Values) ([]byte, error) {
	reqURL := c.baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("FMI returned %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}
```

**Step 2: Verify it compiles**

Run: `cd server && go build ./internal/fmi/`
Expected: No errors.

**Step 3: Commit**

```bash
git add server/internal/fmi/client.go
git commit -m "feat: FMI HTTP client for observations and forecasts"
```

---

## Task 6: PostgreSQL Store

**Files:**
- Create: `server/internal/store/store.go`
- Create: `server/internal/store/store_test.go`

**Dependency:** Requires `github.com/jackc/pgx/v5`

**Step 1: Add pgx dependency**

```bash
cd server && go get github.com/jackc/pgx/v5
```

**Step 2: Write failing test for UpsertStations**

```go
// server/internal/store/store_test.go
package store

import (
	"context"
	"os"
	"testing"

	"github.com/salami-weather/server/internal/weather"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://weather:weather@localhost:5432/weather?sslmode=disable"
	}
	s, err := New(context.Background(), dsn)
	if err != nil {
		t.Skipf("database not available: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertStations(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	stations := []weather.Station{
		{FMISID: 100971, Name: "Helsinki Kaisaniemi", Lat: 60.17523, Lon: 24.94459, WMOCode: "2978"},
	}

	err := s.UpsertStations(ctx, stations)
	if err != nil {
		t.Fatal(err)
	}

	// Query back
	nearest, dist, err := s.NearestStation(ctx, 60.17, 24.94)
	if err != nil {
		t.Fatal(err)
	}
	if nearest.FMISID != 100971 {
		t.Errorf("expected station 100971, got %d", nearest.FMISID)
	}
	if dist > 1.0 {
		t.Errorf("expected distance < 1km, got %f", dist)
	}
}
```

**Step 3: Run test to verify it fails**

Run: `cd server && go test ./internal/store/ -v -run TestUpsertStations`
Expected: FAIL — `Store` type not defined.

**Step 4: Implement the store**

```go
// server/internal/store/store.go
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/salami-weather/server/internal/weather"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) UpsertStations(ctx context.Context, stations []weather.Station) error {
	batch := &pgx.Batch{}
	for _, st := range stations {
		batch.Queue(
			`INSERT INTO stations (fmisid, name, geom, wmo_code)
			 VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography, $5)
			 ON CONFLICT (fmisid) DO UPDATE SET name = $2, geom = ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography, wmo_code = $5`,
			st.FMISID, st.Name, st.Lon, st.Lat, st.WMOCode,
		)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range stations {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upsert station: %w", err)
		}
	}
	return nil
}

func (s *Store) NearestStation(ctx context.Context, lat, lon float64) (weather.Station, float64, error) {
	var st weather.Station
	var distMeters float64
	err := s.pool.QueryRow(ctx,
		`SELECT fmisid, name, ST_Y(geom::geometry), ST_X(geom::geometry), wmo_code,
		        ST_Distance(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)
		 FROM stations
		 ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
		 LIMIT 1`,
		lon, lat,
	).Scan(&st.FMISID, &st.Name, &st.Lat, &st.Lon, &st.WMOCode, &distMeters)
	if err != nil {
		return st, 0, fmt.Errorf("nearest station: %w", err)
	}
	return st, distMeters / 1000.0, nil
}

func (s *Store) UpsertObservations(ctx context.Context, observations []weather.Observation) error {
	batch := &pgx.Batch{}
	for _, o := range observations {
		batch.Queue(
			`INSERT INTO observations (fmisid, observed_at, temperature, wind_speed, wind_dir, humidity, pressure)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (fmisid, observed_at) DO UPDATE SET
			   temperature = $3, wind_speed = $4, wind_dir = $5, humidity = $6, pressure = $7`,
			o.FMISID, o.ObservedAt, o.Temperature, o.WindSpeed, o.WindDir, o.Humidity, o.Pressure,
		)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range observations {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upsert observation: %w", err)
		}
	}
	return nil
}

func (s *Store) LatestObservation(ctx context.Context, fmisid int) (weather.Observation, error) {
	var o weather.Observation
	err := s.pool.QueryRow(ctx,
		`SELECT fmisid, observed_at, temperature, wind_speed, wind_dir, humidity, pressure
		 FROM observations
		 WHERE fmisid = $1
		 ORDER BY observed_at DESC
		 LIMIT 1`,
		fmisid,
	).Scan(&o.FMISID, &o.ObservedAt, &o.Temperature, &o.WindSpeed, &o.WindDir, &o.Humidity, &o.Pressure)
	if err != nil {
		return o, fmt.Errorf("latest observation: %w", err)
	}
	return o, nil
}

func (s *Store) UpsertForecasts(ctx context.Context, forecasts []weather.DailyForecast) error {
	batch := &pgx.Batch{}
	for _, f := range forecasts {
		batch.Queue(
			`INSERT INTO forecasts (grid_lat, grid_lon, forecast_for, fetched_at, temp_high, temp_low, wind_speed, precip_mm, symbol)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 ON CONFLICT (grid_lat, grid_lon, forecast_for) DO UPDATE SET
			   fetched_at = $4, temp_high = $5, temp_low = $6, wind_speed = $7, precip_mm = $8, symbol = $9`,
			f.GridLat, f.GridLon, f.Date, f.FetchedAt, f.TempHigh, f.TempLow, f.WindSpeed, f.PrecipMM, f.Symbol,
		)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range forecasts {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upsert forecast: %w", err)
		}
	}
	return nil
}

func (s *Store) GetForecasts(ctx context.Context, gridLat, gridLon float64) ([]weather.DailyForecast, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT grid_lat, grid_lon, forecast_for, fetched_at, temp_high, temp_low, wind_speed, precip_mm, symbol
		 FROM forecasts
		 WHERE grid_lat = $1 AND grid_lon = $2 AND forecast_for >= CURRENT_DATE
		 ORDER BY forecast_for`,
		gridLat, gridLon,
	)
	if err != nil {
		return nil, fmt.Errorf("get forecasts: %w", err)
	}
	defer rows.Close()

	var result []weather.DailyForecast
	for rows.Next() {
		var f weather.DailyForecast
		if err := rows.Scan(&f.GridLat, &f.GridLon, &f.Date, &f.FetchedAt, &f.TempHigh, &f.TempLow, &f.WindSpeed, &f.PrecipMM, &f.Symbol); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, nil
}
```

**Step 5: Run tests (requires Docker Compose db running)**

Run: `cd server && go test ./internal/store/ -v`
Expected: PASS (or skip if db not available).

**Step 6: Commit**

```bash
git add server/internal/store/
git commit -m "feat: PostgreSQL store with PostGIS spatial queries"
```

---

## Task 7: Background Observation Fetcher

**Files:**
- Create: `server/internal/fetcher/fetcher.go`

**Step 1: Implement the scheduled fetcher**

```go
// server/internal/fetcher/fetcher.go
package fetcher

import (
	"context"
	"log/slog"
	"time"

	"github.com/salami-weather/server/internal/fmi"
	"github.com/salami-weather/server/internal/store"
)

type Fetcher struct {
	fmi   *fmi.Client
	store *store.Store
}

func New(fmiClient *fmi.Client, store *store.Store) *Fetcher {
	return &Fetcher{fmi: fmiClient, store: store}
}

// RunObservationLoop fetches observations on a fixed interval until ctx is cancelled.
func (f *Fetcher) RunObservationLoop(ctx context.Context, interval time.Duration) {
	slog.Info("observation fetcher starting", "interval", interval)

	// Fetch immediately on startup.
	f.fetchObservations(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("observation fetcher stopped")
			return
		case <-ticker.C:
			f.fetchObservations(ctx)
		}
	}
}

func (f *Fetcher) fetchObservations(ctx context.Context) {
	start := time.Now()
	result, err := f.fmi.FetchObservations(ctx)
	if err != nil {
		slog.Error("failed to fetch observations from FMI", "err", err)
		return
	}

	if err := f.store.UpsertStations(ctx, result.Stations); err != nil {
		slog.Error("failed to upsert stations", "err", err)
		return
	}

	if err := f.store.UpsertObservations(ctx, result.Observations); err != nil {
		slog.Error("failed to upsert observations", "err", err)
		return
	}

	slog.Info("observations fetched",
		"stations", len(result.Stations),
		"observations", len(result.Observations),
		"duration", time.Since(start),
	)
}
```

**Step 2: Verify it compiles**

Run: `cd server && go build ./internal/fetcher/`
Expected: No errors.

**Step 3: Commit**

```bash
git add server/internal/fetcher/
git commit -m "feat: background observation fetcher on scheduled interval"
```

---

## Task 8: In-Process Cache + Forecast Service

**Files:**
- Create: `server/internal/weather/cache.go`
- Create: `server/internal/weather/cache_test.go`
- Create: `server/internal/weather/service.go`

**Step 1: Write failing test for TTL cache**

```go
// server/internal/weather/cache_test.go
package weather

import (
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	c := NewCache[string](1 * time.Second)
	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}
}

func TestCache_Expiry(t *testing.T) {
	c := NewCache[string](50 * time.Millisecond)
	c.Set("key1", "value1")

	time.Sleep(100 * time.Millisecond)

	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected cache miss after TTL")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/weather/ -v -run TestCache`
Expected: FAIL — `NewCache` not defined.

**Step 3: Implement the cache**

```go
// server/internal/weather/cache.go
package weather

import (
	"sync"
	"time"
)

type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

type Cache[V any] struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]cacheEntry[V]
}

func NewCache[V any](ttl time.Duration) *Cache[V] {
	return &Cache[V]{
		ttl: ttl,
		m:   make(map[string]cacheEntry[V]),
	}
}

func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.m[key]
	if !ok || time.Now().After(entry.expiresAt) {
		var zero V
		return zero, false
	}
	return entry.value, true
}

func (c *Cache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cacheEntry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
}
```

**Step 4: Run cache tests**

Run: `cd server && go test ./internal/weather/ -v -run TestCache`
Expected: PASS

**Step 5: Implement the weather service**

The service ties together the store, FMI client, and cache to serve the API.

```go
// server/internal/weather/service.go
package weather

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/salami-weather/server/internal/fmi"
	"github.com/salami-weather/server/internal/store"
)

type Service struct {
	store         *store.Store
	fmi           *fmi.Client
	forecastCache *Cache[[]DailyForecast]
}

func NewService(store *store.Store, fmiClient *fmi.Client, forecastCacheTTL time.Duration) *Service {
	return &Service{
		store:         store,
		fmi:           fmiClient,
		forecastCache: NewCache[[]DailyForecast](forecastCacheTTL),
	}
}

// GetWeather returns current conditions + daily forecast for a lat/lon.
func (s *Service) GetWeather(ctx context.Context, lat, lon float64) (*WeatherResponse, error) {
	// 1. Find nearest station and its latest observation.
	station, distKM, err := s.store.NearestStation(ctx, lat, lon)
	if err != nil {
		return nil, fmt.Errorf("nearest station: %w", err)
	}

	obs, err := s.store.LatestObservation(ctx, station.FMISID)
	if err != nil {
		return nil, fmt.Errorf("latest observation: %w", err)
	}

	// 2. Get forecast — check cache, then DB, then FMI.
	gridLat, gridLon := snapToGrid(lat, lon)
	forecast, err := s.getForecast(ctx, gridLat, gridLon)
	if err != nil {
		return nil, fmt.Errorf("forecast: %w", err)
	}

	return &WeatherResponse{
		Current: CurrentWeather{
			Station:     station,
			DistanceKM:  distKM,
			Observation: obs,
		},
		Forecast: forecast,
	}, nil
}

func (s *Service) getForecast(ctx context.Context, gridLat, gridLon float64) ([]DailyForecast, error) {
	cacheKey := fmt.Sprintf("%.2f,%.2f", gridLat, gridLon)

	// Check in-process cache.
	if cached, ok := s.forecastCache.Get(cacheKey); ok {
		return cached, nil
	}

	// Check Postgres.
	forecasts, err := s.store.GetForecasts(ctx, gridLat, gridLon)
	if err == nil && len(forecasts) > 0 && isFresh(forecasts, 3*time.Hour) {
		s.forecastCache.Set(cacheKey, forecasts)
		return forecasts, nil
	}

	// Fetch from FMI.
	forecasts, err = s.fmi.FetchForecast(ctx, gridLat, gridLon)
	if err != nil {
		return nil, err
	}

	// Store in Postgres and cache.
	if storeErr := s.store.UpsertForecasts(ctx, forecasts); storeErr != nil {
		// Log but don't fail — we still have the data.
		fmt.Printf("warning: failed to store forecasts: %v\n", storeErr)
	}
	s.forecastCache.Set(cacheKey, forecasts)

	return forecasts, nil
}

// snapToGrid rounds coordinates to ~1km grid cells (~0.01 degrees).
func snapToGrid(lat, lon float64) (float64, float64) {
	return math.Round(lat*100) / 100, math.Round(lon*100) / 100
}

// isFresh checks whether all forecasts in the batch were fetched within maxAge.
// Uses the oldest fetched_at to avoid partial staleness.
func isFresh(forecasts []DailyForecast, maxAge time.Duration) bool {
	oldest := forecasts[0].FetchedAt
	for _, f := range forecasts[1:] {
		if f.FetchedAt.Before(oldest) {
			oldest = f.FetchedAt
		}
	}
	return time.Since(oldest) < maxAge
}
```

**Step 6: Verify it compiles**

Run: `cd server && go build ./internal/weather/`
Expected: No errors.

**Step 7: Commit**

```bash
git add server/internal/weather/
git commit -m "feat: weather service with in-process forecast cache"
```

---

## Task 9: REST API Handler

**Files:**
- Create: `server/internal/api/handler.go`
- Modify: `server/cmd/server/main.go`

**Step 1: Implement the API handler**

```go
// server/internal/api/handler.go
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/salami-weather/server/internal/weather"
)

type Handler struct {
	service *weather.Service
}

func NewHandler(service *weather.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/weather", h.getWeather)
	mux.HandleFunc("GET /health", h.health)
}

type weatherJSON struct {
	Station  stationJSON         `json:"station"`
	Current  currentJSON         `json:"current"`
	Forecast []dailyForecastJSON `json:"daily_forecast"`
}

type stationJSON struct {
	Name       string  `json:"name"`
	DistanceKM float64 `json:"distance_km"`
}

type currentJSON struct {
	Temperature *float64  `json:"temperature"`
	FeelsLike   *float64  `json:"feels_like"`
	WindSpeed   *float64  `json:"wind_speed"`
	WindDir     *float64  `json:"wind_direction"`
	Humidity    *float64  `json:"humidity"`
	Pressure    *float64  `json:"pressure"`
	ObservedAt  time.Time `json:"observed_at"`
}

type dailyForecastJSON struct {
	Date      string   `json:"date"`
	High      *float64 `json:"high"`
	Low       *float64 `json:"low"`
	Symbol    *string  `json:"symbol"`
	WindSpeed *float64 `json:"wind_speed_avg"`
	PrecipMM  *float64 `json:"precipitation_mm"`
}

func (h *Handler) getWeather(w http.ResponseWriter, r *http.Request) {
	lat, err := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	if err != nil {
		writeJSONError(w, "invalid lat parameter", http.StatusBadRequest)
		return
	}
	lon, err := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	if err != nil {
		writeJSONError(w, "invalid lon parameter", http.StatusBadRequest)
		return
	}

	result, err := h.service.GetWeather(r.Context(), lat, lon)
	if err != nil {
		slog.Error("get weather failed", "err", err, "lat", lat, "lon", lon)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := weatherJSON{
		Station: stationJSON{
			Name:       result.Current.Station.Name,
			DistanceKM: result.Current.DistanceKM,
		},
		Current: currentJSON{
			Temperature: result.Current.Observation.Temperature,
			FeelsLike:   computeFeelsLike(result.Current.Observation.Temperature, result.Current.Observation.WindSpeed),
			WindSpeed:   result.Current.Observation.WindSpeed,
			WindDir:     result.Current.Observation.WindDir,
			Humidity:    result.Current.Observation.Humidity,
			Pressure:    result.Current.Observation.Pressure,
			ObservedAt:  result.Current.Observation.ObservedAt,
		},
	}

	for _, f := range result.Forecast {
		resp.Forecast = append(resp.Forecast, dailyForecastJSON{
			Date:      f.Date.Format("2006-01-02"),
			High:      f.TempHigh,
			Low:       f.TempLow,
			Symbol:    f.Symbol,
			WindSpeed: f.WindSpeed,
			PrecipMM:  f.PrecipMM,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	json.NewEncoder(w).Encode(resp)
}

// computeFeelsLike uses wind chill formula for cold temps.
// https://en.wikipedia.org/wiki/Wind_chill#North_American_and_United_Kingdom_wind_chill_index
func computeFeelsLike(temp, wind *float64) *float64 {
	if temp == nil || wind == nil {
		return temp
	}
	t := *temp
	w := *wind * 3.6 // m/s to km/h
	if t > 10 || w < 4.8 {
		return temp
	}
	fl := 13.12 + 0.6215*t - 11.37*math.Pow(w, 0.16) + 0.3965*t*math.Pow(w, 0.16)
	return &fl
}

func writeJSONError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
```

Note: add `"math"` to the imports.

**Step 2: Wire everything together in main.go**

```go
// server/cmd/server/main.go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/salami-weather/server/internal/api"
	"github.com/salami-weather/server/internal/config"
	"github.com/salami-weather/server/internal/fetcher"
	"github.com/salami-weather/server/internal/fmi"
	"github.com/salami-weather/server/internal/store"
	"github.com/salami-weather/server/internal/weather"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// FMI client
	fmiClient := fmi.NewClient(cfg.FMIBaseURL)

	// Weather service
	svc := weather.NewService(db, fmiClient, 10*time.Minute)

	// Background fetcher
	f := fetcher.New(fmiClient, db)
	go f.RunObservationLoop(ctx, 10*time.Minute)

	// HTTP server
	mux := http.NewServeMux()
	handler := api.NewHandler(svc)
	handler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
	slog.Info("server stopped")
}
```

**Step 3: Verify full build**

Run: `cd server && go build ./cmd/server`
Expected: No errors.

**Step 4: Integration test — start everything and hit the endpoint**

Run: `cd server && docker compose up -d && sleep 5 && curl -s "http://localhost:8080/v1/weather?lat=60.17&lon=24.94" | jq .`
Expected: JSON response with station, current conditions, and forecast. (First request may take a moment while the fetcher populates data.)

**Step 5: Commit**

```bash
git add server/internal/api/ server/cmd/server/main.go
git commit -m "feat: REST API endpoint and wire up all server components"
```

---

## Task 10: iOS Project Setup

**Files:**
- Create: `ios/Weather/WeatherApp.swift`
- Create: `ios/Weather/Models/WeatherData.swift`
- Create: `ios/Weather/Services/WeatherService.swift`
- Create: `ios/Weather/Services/LocationService.swift`
- Create: `ios/Weather.xcodeproj` (via Xcode or xcodegen)

**Step 1: Create the project with xcodegen**

Create `ios/project.yml`:

```yaml
name: Weather
options:
  bundleIdPrefix: com.salami
  deploymentTarget:
    iOS: "26.0"
  xcodeVersion: "26.0"
targets:
  Weather:
    type: application
    platform: iOS
    sources:
      - Weather
    settings:
      base:
        INFOPLIST_KEY_NSLocationWhenInUseUsageDescription: "Weather uses your location to show local conditions and forecasts."
        INFOPLIST_KEY_UILaunchScreen_Generation: true
        SWIFT_VERSION: "6.0"
```

Run: `cd ios && xcodegen generate`

If xcodegen is not installed: `brew install xcodegen && cd ios && xcodegen generate`

**Step 2: Create the app entry point**

```swift
// ios/Weather/WeatherApp.swift
import SwiftUI

@main
struct WeatherApp: App {
    var body: some Scene {
        WindowGroup {
            WeatherView()
        }
    }
}
```

**Step 3: Define models**

```swift
// ios/Weather/Models/WeatherData.swift
import Foundation

struct WeatherResponse: Codable {
    let station: StationInfo
    let current: CurrentConditions
    let dailyForecast: [DailyForecast]

    enum CodingKeys: String, CodingKey {
        case station
        case current
        case dailyForecast = "daily_forecast"
    }
}

struct StationInfo: Codable {
    let name: String
    let distanceKm: Double

    enum CodingKeys: String, CodingKey {
        case name
        case distanceKm = "distance_km"
    }
}

struct CurrentConditions: Codable {
    let temperature: Double?
    let feelsLike: Double?
    let windSpeed: Double?
    let windDirection: Double?
    let humidity: Double?
    let pressure: Double?
    let observedAt: Date

    enum CodingKeys: String, CodingKey {
        case temperature
        case feelsLike = "feels_like"
        case windSpeed = "wind_speed"
        case windDirection = "wind_direction"
        case humidity
        case pressure
        case observedAt = "observed_at"
    }
}

struct DailyForecast: Codable, Identifiable {
    let date: String
    let high: Double?
    let low: Double?
    let symbol: String?
    let windSpeedAvg: Double?
    let precipitationMm: Double?

    var id: String { date }

    var displayDate: Date? {
        let formatter = DateFormatter()
        formatter.dateFormat = "yyyy-MM-dd"
        return formatter.date(from: date)
    }

    enum CodingKeys: String, CodingKey {
        case date, high, low, symbol
        case windSpeedAvg = "wind_speed_avg"
        case precipitationMm = "precipitation_mm"
    }
}
```

**Step 4: Commit**

```bash
git add ios/
git commit -m "feat: scaffold iOS project with models"
```

---

## Task 11: iOS Services

**Files:**
- Create: `ios/Weather/Services/LocationService.swift`
- Create: `ios/Weather/Services/WeatherService.swift`

**Step 1: Implement location service**

```swift
// ios/Weather/Services/LocationService.swift
import CoreLocation

@Observable
final class LocationService: NSObject, CLLocationManagerDelegate {
    private let manager = CLLocationManager()

    var coordinate: CLLocationCoordinate2D?
    var placeName: String?
    var error: Error?

    override init() {
        super.init()
        manager.delegate = self
        manager.desiredAccuracy = kCLLocationAccuracyKilometer
    }

    func requestLocation() {
        switch manager.authorizationStatus {
        case .notDetermined:
            manager.requestWhenInUseAuthorization()
            // requestLocation() will be called from locationManagerDidChangeAuthorization once granted.
        case .authorizedWhenInUse, .authorizedAlways:
            manager.requestLocation()
        default:
            break
        }
    }

    func locationManagerDidChangeAuthorization(_ manager: CLLocationManager) {
        switch manager.authorizationStatus {
        case .authorizedWhenInUse, .authorizedAlways:
            manager.requestLocation()
        default:
            break
        }
    }

    func locationManager(_ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        guard let location = locations.last else { return }
        coordinate = location.coordinate
        reverseGeocode(location)
    }

    func locationManager(_ manager: CLLocationManager, didFailWithError error: Error) {
        self.error = error
    }

    private func reverseGeocode(_ location: CLLocation) {
        CLGeocoder().reverseGeocodeLocation(location) { placemarks, _ in
            self.placeName = placemarks?.first?.locality
        }
    }
}
```

**Step 2: Implement weather service**

```swift
// ios/Weather/Services/WeatherService.swift
import Foundation

actor WeatherService {
    // TODO: Change to your server's URL before deploying.
    private let baseURL = URL(string: "http://localhost:8080")!

    func fetchWeather(lat: Double, lon: Double) async throws -> WeatherResponse {
        var components = URLComponents(url: baseURL.appendingPathComponent("v1/weather"), resolvingAgainstBaseURL: false)!
        components.queryItems = [
            URLQueryItem(name: "lat", value: String(format: "%.4f", lat)),
            URLQueryItem(name: "lon", value: String(format: "%.4f", lon)),
        ]

        let (data, response) = try await URLSession.shared.data(from: components.url!)

        guard let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode == 200 else {
            throw WeatherError.serverError
        }

        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try decoder.decode(WeatherResponse.self, from: data)
    }

    // Offline cache: save/load last response.
    func saveToCache(_ response: WeatherResponse) {
        guard let data = try? JSONEncoder().encode(response) else { return }
        try? data.write(to: cacheURL)
    }

    func loadFromCache() -> WeatherResponse? {
        guard let data = try? Data(contentsOf: cacheURL) else { return nil }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try? decoder.decode(WeatherResponse.self, from: data)
    }

    private var cacheURL: URL {
        FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("weather_cache.json")
    }
}

enum WeatherError: Error {
    case serverError
}
```

**Step 3: Commit**

```bash
git add ios/Weather/Services/
git commit -m "feat: iOS location and weather services"
```

---

## Task 12: iOS Weather UI

**Files:**
- Create: `ios/Weather/Views/WeatherView.swift`
- Create: `ios/Weather/Views/CurrentConditionsCard.swift`
- Create: `ios/Weather/Views/DailyForecastRow.swift`

**Step 1: Create the main view**

```swift
// ios/Weather/Views/WeatherView.swift
import SwiftUI

struct WeatherView: View {
    @State private var locationService = LocationService()
    @State private var weatherService = WeatherService()
    @State private var weather: WeatherResponse?
    @State private var isLoading = false
    @State private var lastUpdated: Date?

    var body: some View {
        ScrollView {
            VStack(spacing: 20) {
                if let weather {
                    headerSection(weather)
                    CurrentConditionsCard(current: weather.current)
                    dailyForecastSection(weather.dailyForecast)
                    if let lastUpdated {
                        Text("Updated \(lastUpdated, style: .relative) ago")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                } else if isLoading {
                    ProgressView("Loading weather...")
                        .padding(.top, 100)
                } else {
                    ContentUnavailableView(
                        "No Weather Data",
                        systemImage: "cloud.slash",
                        description: Text("Pull down to refresh")
                    )
                }
            }
            .padding()
        }
        .refreshable { await loadWeather() }
        .task {
            locationService.requestLocation()
            await loadWeather()
        }
        .onChange(of: locationService.coordinate?.latitude) {
            // Re-fetch when coordinate becomes available (e.g., after permission grant).
            Task { await loadWeather() }
        }
    }

    @ViewBuilder
    private func headerSection(_ weather: WeatherResponse) -> some View {
        VStack(spacing: 4) {
            Text(locationService.placeName ?? weather.station.name)
                .font(.title2)
            if let temp = weather.current.temperature {
                Text("\(Int(temp.rounded()))°")
                    .font(.system(size: 72, weight: .thin))
            }
            if let feelsLike = weather.current.feelsLike {
                Text("Feels like \(Int(feelsLike.rounded()))°")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.top, 20)
    }

    @ViewBuilder
    private func dailyForecastSection(_ forecasts: [DailyForecast]) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            Label("10-DAY FORECAST", systemImage: "calendar")
                .font(.caption)
                .foregroundStyle(.secondary)
                .padding(.bottom, 8)

            ForEach(forecasts) { day in
                DailyForecastRow(
                    forecast: day,
                    overallLow: forecasts.compactMap(\.low).min() ?? 0,
                    overallHigh: forecasts.compactMap(\.high).max() ?? 0
                )
                if day.id != forecasts.last?.id {
                    Divider()
                }
            }
        }
        .padding()
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 12))
    }

    private func loadWeather() async {
        guard let coord = locationService.coordinate else {
            // Show cached data while waiting for location permission/fix.
            if weather == nil {
                weather = await weatherService.loadFromCache()
            }
            return
        }
        await fetchWeather(lat: coord.latitude, lon: coord.longitude)
    }

    private func fetchWeather(lat: Double, lon: Double) async {
        isLoading = true
        defer { isLoading = false }

        do {
            let response = try await weatherService.fetchWeather(lat: lat, lon: lon)
            weather = response
            lastUpdated = Date()
            await weatherService.saveToCache(response)
        } catch {
            // Show cached data if available.
            if weather == nil {
                weather = await weatherService.loadFromCache()
            }
        }
    }
}
```

**Step 2: Current conditions card**

```swift
// ios/Weather/Views/CurrentConditionsCard.swift
import SwiftUI

struct CurrentConditionsCard: View {
    let current: CurrentConditions

    var body: some View {
        LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible())], spacing: 12) {
            conditionItem(title: "WIND", value: formatWind(), icon: "wind")
            conditionItem(title: "HUMIDITY", value: formatPercent(current.humidity), icon: "humidity")
            conditionItem(title: "PRESSURE", value: formatPressure(), icon: "gauge.medium")
            conditionItem(title: "WIND DIR", value: formatWindDir(), icon: "location.north.line")
        }
        .padding()
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 12))
    }

    @ViewBuilder
    private func conditionItem(title: String, value: String, icon: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Label(title, systemImage: icon)
                .font(.caption)
                .foregroundStyle(.secondary)
            Text(value)
                .font(.title3)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func formatWind() -> String {
        guard let speed = current.windSpeed else { return "--" }
        return String(format: "%.0f m/s", speed)
    }

    private func formatPercent(_ value: Double?) -> String {
        guard let value else { return "--" }
        return "\(Int(value))%"
    }

    private func formatPressure() -> String {
        guard let p = current.pressure else { return "--" }
        return String(format: "%.0f hPa", p)
    }

    private func formatWindDir() -> String {
        guard let dir = current.windDirection else { return "--" }
        let directions = ["N", "NE", "E", "SE", "S", "SW", "W", "NW"]
        let index = Int((dir + 22.5) / 45.0) % 8
        return directions[index]
    }
}
```

**Step 3: Daily forecast row**

```swift
// ios/Weather/Views/DailyForecastRow.swift
import SwiftUI

struct DailyForecastRow: View {
    let forecast: DailyForecast
    let overallLow: Double
    let overallHigh: Double

    var body: some View {
        HStack {
            Text(dayName)
                .frame(width: 44, alignment: .leading)

            // Weather symbol placeholder — replace with proper icons later.
            Image(systemName: symbolName)
                .frame(width: 30)
                .symbolRenderingMode(.multicolor)

            Text(formatTemp(forecast.low))
                .foregroundStyle(.secondary)
                .frame(width: 36, alignment: .trailing)

            temperatureBar
                .frame(height: 4)

            Text(formatTemp(forecast.high))
                .frame(width: 36, alignment: .trailing)
        }
        .padding(.vertical, 8)
    }

    private var dayName: String {
        guard let date = forecast.displayDate else { return forecast.date }
        if Calendar.current.isDateInToday(date) { return "Today" }
        let formatter = DateFormatter()
        formatter.dateFormat = "EEE"
        return formatter.string(from: date)
    }

    private var symbolName: String {
        // Map FMI weather symbol codes to SF Symbols.
        // This is a simplified mapping — expand as needed.
        switch forecast.symbol {
        case "1": return "sun.max.fill"
        case "2": return "cloud.sun.fill"
        case "3": return "cloud.fill"
        case "21", "22", "23": return "cloud.rain.fill"
        case "41", "42", "43": return "cloud.snow.fill"
        case "61", "62", "63": return "cloud.bolt.rain.fill"
        default: return "cloud.fill"
        }
    }

    @ViewBuilder
    private var temperatureBar: some View {
        GeometryReader { geo in
            let range = overallHigh - overallLow
            let lo = forecast.low ?? overallLow
            let hi = forecast.high ?? overallHigh

            let startFraction = range > 0 ? (lo - overallLow) / range : 0
            let endFraction = range > 0 ? (hi - overallLow) / range : 1

            Capsule()
                .fill(.gray.opacity(0.2))
                .overlay(alignment: .leading) {
                    Capsule()
                        .fill(temperatureGradient)
                        .frame(width: geo.size.width * (endFraction - startFraction))
                        .offset(x: geo.size.width * startFraction)
                }
        }
    }

    private var temperatureGradient: LinearGradient {
        LinearGradient(
            colors: [.blue, .green, .yellow, .orange, .red],
            startPoint: .leading,
            endPoint: .trailing
        )
    }

    private func formatTemp(_ temp: Double?) -> String {
        guard let temp else { return "--" }
        return "\(Int(temp.rounded()))°"
    }
}
```

**Step 4: Build the iOS project**

Run: `cd ios && xcodebuild -scheme Weather -destination 'platform=iOS Simulator,name=iPhone 16' build`
Expected: BUILD SUCCEEDED

**Step 5: Commit**

```bash
git add ios/Weather/Views/
git commit -m "feat: iOS weather UI with current conditions and daily forecast"
```

---

## Task 13: Deployment Configuration

**Files:**
- Create: `server/Caddyfile`
- Modify: `server/docker-compose.yml`

**Step 1: Create Caddyfile**

```
# server/Caddyfile
# Replace with your actual domain.
api.yourweatherapp.fi {
    reverse_proxy server:8080
}
```

**Step 2: Add Caddy to Docker Compose**

Add to `server/docker-compose.yml`:

```yaml
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      - server
```

Add to volumes:

```yaml
  caddy_data:
  caddy_config:
```

**Step 3: Commit**

```bash
git add server/Caddyfile server/docker-compose.yml
git commit -m "feat: add Caddy reverse proxy for HTTPS"
```

---

## Task 14: End-to-End Smoke Test

**Step 1: Start everything locally**

```bash
cd server && docker compose up --build -d
```

**Step 2: Wait for fetcher to populate data**

```bash
sleep 15 && docker compose logs server --tail 20
```

Expected: Log lines showing "observations fetched" with station/observation counts.

**Step 3: Hit the API**

```bash
curl -s "http://localhost:8080/v1/weather?lat=60.17&lon=24.94" | jq .
```

Expected: Full JSON response with station info, current conditions, and daily forecast.

**Step 4: Run the iOS app in simulator**

Open `ios/Weather.xcodeproj` in Xcode, run on iOS simulator. The app should show weather data from the local server. (Ensure simulator can reach `localhost:8080`.)

**Step 5: Commit any fixes, tag v1-alpha**

```bash
git tag v0.1.0-alpha
```
