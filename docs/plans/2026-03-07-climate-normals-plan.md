# Climate Normals Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Serve FMI 30-year climate normals (1991-2020) via a dedicated API endpoint with cosine-interpolated daily values, and display them in a new iOS card.

**Architecture:** New Postgres table stores 12 monthly normals per station. A one-time import CLI fetches data from FMI WFS. A new `GET /v1/climate-normals` endpoint finds the nearest station, interpolates to today, and returns both the daily snapshot and full monthly array. iOS fetches once, caches indefinitely, and renders a ClimateNormalsCard.

**Tech Stack:** Go 1.26, PostgreSQL 18 + PostGIS, SwiftUI (iOS 26), FMI WFS API

---

### Task 1: Database Migration

**Files:**
- Create: `server/migrations/007_climate_normals.sql`

**Step 1: Write migration**

```sql
CREATE TABLE IF NOT EXISTS climate_normals (
    fmisid    INTEGER NOT NULL REFERENCES stations(fmisid),
    month     SMALLINT NOT NULL CHECK (month BETWEEN 1 AND 12),
    period    TEXT NOT NULL DEFAULT '1991-2020',
    temp_avg  DOUBLE PRECISION,
    temp_high DOUBLE PRECISION,
    temp_low  DOUBLE PRECISION,
    precip_mm DOUBLE PRECISION,
    PRIMARY KEY (fmisid, month, period)
);
```

**Step 2: Verify migration applies**

Run: `cd server && ./scripts/local-dev.sh up`
Expected: Server starts, table exists. Verify with:
```bash
psql -c "SELECT * FROM climate_normals LIMIT 0;"
```

**Step 3: Commit**

```bash
git add server/migrations/007_climate_normals.sql
git commit -m "server: add climate_normals table migration"
```

---

### Task 2: Climate Normals Domain Model

**Files:**
- Modify: `server/internal/weather/models.go`

**Step 1: Add ClimateNormal and ClimateNormalsResponse structs**

Add to `models.go`:

```go
type ClimateNormal struct {
	FMISID   int
	Month    int
	Period   string
	TempAvg  *float64
	TempHigh *float64
	TempLow  *float64
	PrecipMm *float64
}

type InterpolatedNormal struct {
	TempAvg     *float64
	TempHigh    *float64
	TempLow     *float64
	PrecipMmDay *float64
	TempDiff    *float64
}
```

**Step 2: Run build to verify**

Run: `cd server && /usr/local/go/bin/go build ./...`
Expected: Clean build

**Step 3: Commit**

```bash
git add server/internal/weather/models.go
git commit -m "server: add climate normal domain models"
```

---

### Task 3: XML Parser for Climate Normals

**Files:**
- Modify: `server/internal/fmi/parser.go`
- Test: `server/internal/fmi/parser_test.go`
- Testdata: `server/internal/fmi/testdata/climate_normals.xml` (already exists)

**Step 1: Write the failing test**

Add to `parser_test.go`:

```go
func TestParseClimateNormals(t *testing.T) {
	data, err := os.ReadFile("testdata/climate_normals.xml")
	if err != nil {
		t.Fatal(err)
	}

	normals, err := ParseClimateNormals(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(normals) == 0 {
		t.Fatal("expected at least one climate normal entry")
	}

	// Should have 12 months for the station
	byMonth := make(map[int]weather.ClimateNormal)
	for _, n := range normals {
		byMonth[n.Month] = n
	}
	if len(byMonth) != 12 {
		t.Fatalf("expected 12 months, got %d", len(byMonth))
	}

	// January should have sub-zero temp_avg for Helsinki
	jan, ok := byMonth[1]
	if !ok {
		t.Fatal("missing January")
	}
	if jan.TempAvg == nil {
		t.Fatal("January temp_avg should not be nil")
	}
	if *jan.TempAvg > 5 {
		t.Errorf("Helsinki January average should be below 5°C, got %f", *jan.TempAvg)
	}
	if jan.FMISID != 100971 {
		t.Errorf("expected FMISID 100971, got %d", jan.FMISID)
	}

	// July should have positive temp and precipitation
	jul := byMonth[7]
	if jul.TempAvg == nil || *jul.TempAvg < 10 {
		t.Errorf("Helsinki July average should be above 10°C, got %v", jul.TempAvg)
	}
	if jul.TempHigh == nil {
		t.Error("July temp_high should not be nil")
	}
	if jul.TempLow == nil {
		t.Error("July temp_low should not be nil")
	}
	if jul.PrecipMm == nil {
		t.Error("July precip_mm should not be nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd server && /usr/local/go/bin/go test ./internal/fmi -run TestParseClimateNormals -v`
Expected: FAIL — `ParseClimateNormals` undefined

**Step 3: Implement ParseClimateNormals**

Add to `parser.go`. The climate normals XML uses the same timevaluepair format as observations. Each `<om:member>` contains one parameter (e.g., `TAP1M`) with 12 `<wml2:MeasurementTVP>` entries (one per month, timestamped at the 1st of each month in 1991).

The function should:
1. Unmarshal the XML using the existing `featureCollection` / `member` structures
2. Extract FMISID from the station identifier URL (same pattern as `ParseObservations`)
3. For each member, extract the parameter name from `observedProperty.Href` (look for `param=`)
4. Iterate the time-value pairs, extract month from the timestamp
5. Build a map of `(fmisid, month) -> ClimateNormal`, filling in the relevant field based on parameter name:
   - `TAP1M` -> `TempAvg`
   - `TAMAXP1M` -> `TempHigh`
   - `TAMINP1M` -> `TempLow`
   - `PRAP1M` -> `PrecipMm`
6. Return the map values as a slice

```go
func ParseClimateNormals(data []byte) ([]weather.ClimateNormal, error)
```

**Step 4: Run test to verify it passes**

Run: `cd server && /usr/local/go/bin/go test ./internal/fmi -run TestParseClimateNormals -v`
Expected: PASS

**Step 5: Commit**

```bash
git add server/internal/fmi/parser.go server/internal/fmi/parser_test.go
git commit -m "server: parse FMI 30-year climate normals XML"
```

---

### Task 4: Store — Upsert and Query Climate Normals

**Files:**
- Modify: `server/internal/store/store.go`
- Modify: `server/internal/weather/service.go` (add method to `WeatherStore` interface)

**Step 1: Add methods to WeatherStore interface**

In `server/internal/weather/service.go`, add to `WeatherStore`:

```go
UpsertClimateNormals(ctx context.Context, normals []ClimateNormal) error
GetClimateNormals(ctx context.Context, fmisid int, period string) ([]ClimateNormal, error)
```

**Step 2: Implement in store.go**

`UpsertClimateNormals` — batch upsert using `pgx.Batch`:

```go
func (s *Store) UpsertClimateNormals(ctx context.Context, normals []weather.ClimateNormal) error {
	batch := &pgx.Batch{}
	for _, n := range normals {
		batch.Queue(`
			INSERT INTO climate_normals (fmisid, month, period, temp_avg, temp_high, temp_low, precip_mm)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (fmisid, month, period) DO UPDATE SET
				temp_avg = EXCLUDED.temp_avg,
				temp_high = EXCLUDED.temp_high,
				temp_low = EXCLUDED.temp_low,
				precip_mm = EXCLUDED.precip_mm`,
			n.FMISID, n.Month, n.Period, n.TempAvg, n.TempHigh, n.TempLow, n.PrecipMm)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range normals {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upsert climate normals: %w", err)
		}
	}
	return nil
}
```

`GetClimateNormals` — query by FMISID and period, ordered by month:

```go
func (s *Store) GetClimateNormals(ctx context.Context, fmisid int, period string) ([]weather.ClimateNormal, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT fmisid, month, period, temp_avg, temp_high, temp_low, precip_mm
		FROM climate_normals
		WHERE fmisid = $1 AND period = $2
		ORDER BY month`, fmisid, period)
	if err != nil {
		return nil, fmt.Errorf("get climate normals: %w", err)
	}
	defer rows.Close()

	var normals []weather.ClimateNormal
	for rows.Next() {
		var n weather.ClimateNormal
		if err := rows.Scan(&n.FMISID, &n.Month, &n.Period, &n.TempAvg, &n.TempHigh, &n.TempLow, &n.PrecipMm); err != nil {
			return nil, fmt.Errorf("scan climate normal: %w", err)
		}
		normals = append(normals, n)
	}
	return normals, rows.Err()
}
```

**Step 3: Run build to verify**

Run: `cd server && /usr/local/go/bin/go build ./...`
Expected: Clean build

**Step 4: Commit**

```bash
git add server/internal/weather/service.go server/internal/store/store.go
git commit -m "server: add climate normals store operations"
```

---

### Task 5: Cosine Interpolation

**Files:**
- Create: `server/internal/weather/interpolate.go`
- Create: `server/internal/weather/interpolate_test.go`

**Step 1: Write the failing tests**

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `cd server && /usr/local/go/bin/go test ./internal/weather -run TestInterpolateNormals -v`
Expected: FAIL — `InterpolateNormals` undefined

**Step 3: Implement InterpolateNormals**

In `interpolate.go`:

```go
package weather

import (
	"math"
	"time"
)

// InterpolateNormals computes a daily interpolated climate normal from 12 monthly values
// using cosine interpolation. Each monthly value is placed at the 15th of its month.
// currentTemp is used to compute TempDiff; pass nil if unavailable.
func InterpolateNormals(normals []ClimateNormal, date time.Time, currentTemp *float64) InterpolatedNormal {
	if len(normals) != 12 {
		return InterpolatedNormal{}
	}

	// Sort by month (1-12) into a fixed array
	var monthly [12]ClimateNormal
	for _, n := range normals {
		if n.Month >= 1 && n.Month <= 12 {
			monthly[n.Month-1] = n
		}
	}

	result := InterpolatedNormal{
		TempAvg:     interpolateField(monthly, date, func(n ClimateNormal) *float64 { return n.TempAvg }),
		TempHigh:    interpolateField(monthly, date, func(n ClimateNormal) *float64 { return n.TempHigh }),
		TempLow:     interpolateField(monthly, date, func(n ClimateNormal) *float64 { return n.TempLow }),
		PrecipMmDay: interpolatePrecip(monthly, date),
	}

	if currentTemp != nil && result.TempAvg != nil {
		diff := *currentTemp - *result.TempAvg
		result.TempDiff = &diff
	}

	return result
}

func interpolateField(monthly [12]ClimateNormal, date time.Time, getter func(ClimateNormal) *float64) *float64 {
	year := date.Year()
	dayOfYear := date.YearDay()

	// Find the two surrounding mid-month points
	month := date.Month()
	midCurrent := midMonthDay(year, month)

	var beforeMonth, afterMonth time.Month
	if dayOfYear >= midCurrent {
		beforeMonth = month
		afterMonth = month + 1
		if afterMonth > 12 {
			afterMonth = 1
		}
	} else {
		afterMonth = month
		beforeMonth = month - 1
		if beforeMonth < 1 {
			beforeMonth = 12
		}
	}

	valBefore := getter(monthly[beforeMonth-1])
	valAfter := getter(monthly[afterMonth-1])
	if valBefore == nil || valAfter == nil {
		return nil
	}

	midBefore := midMonthDay(year, beforeMonth)
	midAfter := midMonthDay(year, afterMonth)

	// Handle year wrap (Dec -> Jan)
	doy := float64(dayOfYear)
	mb := float64(midBefore)
	ma := float64(midAfter)
	if ma <= mb {
		daysInYear := float64(daysInYear(year))
		if doy < mb {
			doy += daysInYear
		}
		ma += daysInYear
	}

	t := (doy - mb) / (ma - mb)
	weight := (1 - math.Cos(t*math.Pi)) / 2
	v := *valBefore*(1-weight) + *valAfter*weight
	return &v
}

func interpolatePrecip(monthly [12]ClimateNormal, date time.Time) *float64 {
	// Interpolate monthly total, then divide by days in the interpolated month range
	totalPtr := interpolateField(monthly, date, func(n ClimateNormal) *float64 { return n.PrecipMm })
	if totalPtr == nil {
		return nil
	}
	days := float64(daysInMonth(date.Year(), date.Month()))
	v := *totalPtr / days
	return &v
}

func midMonthDay(year int, month time.Month) int {
	return time.Date(year, month, 15, 0, 0, 0, 0, time.UTC).YearDay()
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func daysInYear(year int) int {
	return time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC).YearDay()
}
```

**Step 4: Run tests to verify they pass**

Run: `cd server && /usr/local/go/bin/go test ./internal/weather -run TestInterpolateNormals -v`
Expected: PASS

**Step 5: Commit**

```bash
git add server/internal/weather/interpolate.go server/internal/weather/interpolate_test.go
git commit -m "server: add cosine interpolation for climate normals"
```

---

### Task 6: Service Method — GetClimateNormals

**Files:**
- Modify: `server/internal/weather/service.go`

**Step 1: Add GetClimateNormals to Service**

```go
func (s *Service) GetClimateNormals(ctx context.Context, lat, lon float64, currentTemp *float64) (*Station, float64, []ClimateNormal, InterpolatedNormal, error) {
	station, distKm, err := s.store.NearestStation(ctx, lat, lon)
	if err != nil {
		return nil, 0, nil, InterpolatedNormal{}, fmt.Errorf("nearest station: %w", err)
	}

	normals, err := s.store.GetClimateNormals(ctx, station.FMISID, "1991-2020")
	if err != nil {
		return nil, 0, nil, InterpolatedNormal{}, fmt.Errorf("get climate normals: %w", err)
	}

	today := InterpolateNormals(normals, time.Now().UTC(), currentTemp)
	return &station, distKm, normals, today, nil
}
```

**Step 2: Run build**

Run: `cd server && /usr/local/go/bin/go build ./...`
Expected: Clean build

**Step 3: Commit**

```bash
git add server/internal/weather/service.go
git commit -m "server: add GetClimateNormals service method"
```

---

### Task 7: API Endpoint — GET /v1/climate-normals

**Files:**
- Modify: `server/internal/api/handler.go`

**Step 1: Add handler and register route**

Add to `RegisterRoutes`:
```go
mux.HandleFunc("GET /v1/climate-normals", h.getClimateNormals)
```

Add handler method and JSON response types:

```go
type climateNormalsJSON struct {
	Station stationJSON              `json:"station"`
	Period  string                   `json:"period"`
	Today   interpolatedNormalJSON   `json:"today"`
	Monthly []monthlyNormalJSON      `json:"monthly"`
}

type interpolatedNormalJSON struct {
	TempAvg     *float64 `json:"temp_avg"`
	TempHigh    *float64 `json:"temp_high"`
	TempLow     *float64 `json:"temp_low"`
	PrecipMmDay *float64 `json:"precip_mm_day"`
	TempDiff    *float64 `json:"temp_diff"`
}

type monthlyNormalJSON struct {
	Month    int      `json:"month"`
	TempAvg  *float64 `json:"temp_avg"`
	TempHigh *float64 `json:"temp_high"`
	TempLow  *float64 `json:"temp_low"`
	PrecipMm *float64 `json:"precip_mm"`
}

func (h *Handler) getClimateNormals(w http.ResponseWriter, r *http.Request) {
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

	// Optionally accept current_temp for computing temp_diff
	var currentTemp *float64
	if ct := r.URL.Query().Get("current_temp"); ct != "" {
		if v, err := strconv.ParseFloat(ct, 64); err == nil {
			currentTemp = &v
		}
	}

	station, distKm, normals, today, err := h.service.GetClimateNormals(r.Context(), lat, lon, currentTemp)
	if err != nil {
		slog.Error("climate normals", "err", err)
		writeJSONError(w, "failed to get climate normals", http.StatusInternalServerError)
		return
	}

	monthly := make([]monthlyNormalJSON, len(normals))
	for i, n := range normals {
		monthly[i] = monthlyNormalJSON{
			Month:    n.Month,
			TempAvg:  n.TempAvg,
			TempHigh: n.TempHigh,
			TempLow:  n.TempLow,
			PrecipMm: n.PrecipMm,
		}
	}

	resp := climateNormalsJSON{
		Station: stationJSON{Name: station.Name, DistanceKm: distKm},
		Period:  "1991-2020",
		Today: interpolatedNormalJSON{
			TempAvg:     today.TempAvg,
			TempHigh:    today.TempHigh,
			TempLow:     today.TempLow,
			PrecipMmDay: today.PrecipMmDay,
			TempDiff:    today.TempDiff,
		},
		Monthly: monthly,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	json.NewEncoder(w).Encode(resp)
}
```

**Step 2: Run build**

Run: `cd server && /usr/local/go/bin/go build ./...`
Expected: Clean build

**Step 3: Commit**

```bash
git add server/internal/api/handler.go
git commit -m "server: add GET /v1/climate-normals endpoint"
```

---

### Task 8: Import CLI

**Files:**
- Create: `server/cmd/import-normals/main.go`

**Step 1: Write the import command**

This CLI:
1. Connects to Postgres
2. Queries all station FMISIDs from `stations` table
3. Batches them into groups of ~20 (to avoid overwhelming FMI API)
4. For each batch, calls FMI WFS with comma-separated FMISIDs
5. Parses the XML response
6. Upserts into `climate_normals`

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"wby/internal/fmi"
	"wby/internal/store"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL not set")
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := store.New(ctx, dsn)
	if err != nil {
		slog.Error("connect to database", "err", err)
		os.Exit(1)
	}

	fmiBaseURL := os.Getenv("FMI_BASE_URL")
	if fmiBaseURL == "" {
		fmiBaseURL = "https://opendata.fmi.fi/wfs"
	}

	stationIDs, err := db.AllStationFMISIDs(ctx)
	if err != nil {
		slog.Error("list stations", "err", err)
		os.Exit(1)
	}
	slog.Info("found stations", "count", len(stationIDs))

	client := fmi.NewClient(fmiBaseURL, "", "")
	batchSize := 20
	total := 0

	for i := 0; i < len(stationIDs); i += batchSize {
		end := i + batchSize
		if end > len(stationIDs) {
			end = len(stationIDs)
		}
		batch := stationIDs[i:end]

		fmisidStrs := make([]string, len(batch))
		for j, id := range batch {
			fmisidStrs[j] = fmt.Sprintf("%d", id)
		}

		data, err := client.FetchClimateNormals(ctx, strings.Join(fmisidStrs, ","))
		if err != nil {
			slog.Warn("fetch batch", "start", i, "err", err)
			continue
		}

		normals, err := fmi.ParseClimateNormals(data)
		if err != nil {
			slog.Warn("parse batch", "start", i, "err", err)
			continue
		}

		if err := db.UpsertClimateNormals(ctx, normals); err != nil {
			slog.Warn("upsert batch", "start", i, "err", err)
			continue
		}

		total += len(normals)
		slog.Info("imported batch", "stations", len(batch), "normals", len(normals), "total", total)

		// Rate limit: be nice to FMI API
		time.Sleep(1 * time.Second)
	}

	slog.Info("import complete", "total_normals", total)
}
```

**Step 2: Add supporting methods**

Add `AllStationFMISIDs` to `store.go`:
```go
func (s *Store) AllStationFMISIDs(ctx context.Context) ([]int, error) {
	rows, err := s.pool.Query(ctx, "SELECT fmisid FROM stations ORDER BY fmisid")
	if err != nil {
		return nil, fmt.Errorf("list station fmisids: %w", err)
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan fmisid: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
```

Add `FetchClimateNormals` to `client.go`:
```go
func (c *Client) FetchClimateNormals(ctx context.Context, fmisids string) ([]byte, error) {
	params := url.Values{}
	params.Set("service", "WFS")
	params.Set("version", "2.0.0")
	params.Set("request", "getFeature")
	params.Set("storedquery_id", "fmi::observations::weather::monthly::30year::timevaluepair")
	params.Set("fmisid", fmisids)
	params.Set("starttime", "1991-01-01T00:00:00Z")

	reqURL := c.baseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch climate normals: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
```

**Step 3: Run build**

Run: `cd server && /usr/local/go/bin/go build ./cmd/import-normals`
Expected: Clean build

**Step 4: Commit**

```bash
git add server/cmd/import-normals/ server/internal/store/store.go server/internal/fmi/client.go
git commit -m "server: add climate normals import CLI"
```

---

### Task 9: iOS Model — ClimateNormalsResponse

**Files:**
- Create: `ios/wby/wby/Models/ClimateNormalsData.swift`

**Step 1: Write the model**

```swift
struct ClimateNormalsResponse: Codable {
    let station: StationInfo
    let period: String
    let today: InterpolatedNormal
    let monthly: [MonthlyNormal]
}

struct InterpolatedNormal: Codable {
    let tempAvg: Double?
    let tempHigh: Double?
    let tempLow: Double?
    let precipMmDay: Double?
    let tempDiff: Double?

    enum CodingKeys: String, CodingKey {
        case tempAvg = "temp_avg"
        case tempHigh = "temp_high"
        case tempLow = "temp_low"
        case precipMmDay = "precip_mm_day"
        case tempDiff = "temp_diff"
    }
}

struct MonthlyNormal: Codable, Identifiable {
    let month: Int
    let tempAvg: Double?
    let tempHigh: Double?
    let tempLow: Double?
    let precipMm: Double?

    var id: Int { month }

    enum CodingKeys: String, CodingKey {
        case month
        case tempAvg = "temp_avg"
        case tempHigh = "temp_high"
        case tempLow = "temp_low"
        case precipMm = "precip_mm"
    }
}
```

**Step 2: Build to verify**

Run: `xcodebuild -project ios/wby/wby.xcodeproj -scheme wby -destination 'generic/platform=iOS Simulator' build`
Expected: BUILD SUCCEEDED

**Step 3: Commit**

```bash
git add ios/wby/wby/Models/ClimateNormalsData.swift
git commit -m "ios: add climate normals response models"
```

---

### Task 10: iOS Service — Fetch and Cache Climate Normals

**Files:**
- Modify: `ios/wby/wby/Services/WeatherService.swift`

**Step 1: Add fetch and cache methods**

Add to `WeatherService`:

```swift
func fetchClimateNormals(lat: Double, lon: Double) async throws -> ClimateNormalsResponse {
    guard let baseURL, let clientID, let clientSecret else {
        throw URLError(.badURL)
    }

    let query = "lat=\(lat)&lon=\(lon)"
    let url = baseURL.appendingPathComponent("/v1/climate-normals")
    var components = URLComponents(url: url, resolvingAgainstBaseURL: false)!
    components.query = query
    guard let finalURL = components.url else { throw URLError(.badURL) }

    let request = try signedRequest(url: finalURL, method: "GET", query: query)
    let (data, _) = try await URLSession.shared.data(for: request)

    let decoder = JSONDecoder()
    return try decoder.decode(ClimateNormalsResponse.self, from: data)
}

private func climateNormalsCacheURL(lat: Double, lon: Double) -> URL {
    let latStr = String(format: "%.2f", locale: Locale(identifier: "en_US_POSIX"), lat)
    let lonStr = String(format: "%.2f", locale: Locale(identifier: "en_US_POSIX"), lon)
    return FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask)[0]
        .appendingPathComponent("climate_normals_\(latStr)_\(lonStr).json")
}

func fetchAndCacheClimateNormals(lat: Double, lon: Double) async throws -> ClimateNormalsResponse {
    let response = try await fetchClimateNormals(lat: lat, lon: lon)
    let data = try JSONEncoder().encode(response)
    try? data.write(to: climateNormalsCacheURL(lat: lat, lon: lon))
    return response
}

func loadClimateNormalsFromCache(lat: Double, lon: Double) -> ClimateNormalsResponse? {
    let url = climateNormalsCacheURL(lat: lat, lon: lon)
    guard let data = try? Data(contentsOf: url) else { return nil }
    return try? JSONDecoder().decode(ClimateNormalsResponse.self, from: data)
}

func loadClimateNormalsFromCacheOrFetch(lat: Double, lon: Double) async -> ClimateNormalsResponse? {
    if let cached = loadClimateNormalsFromCache(lat: lat, lon: lon) {
        return cached
    }
    return try? await fetchAndCacheClimateNormals(lat: lat, lon: lon)
}
```

**Step 2: Build to verify**

Run: `xcodebuild -project ios/wby/wby.xcodeproj -scheme wby -destination 'generic/platform=iOS Simulator' build`
Expected: BUILD SUCCEEDED

**Step 3: Commit**

```bash
git add ios/wby/wby/Services/WeatherService.swift
git commit -m "ios: add climate normals fetch and cache"
```

---

### Task 11: iOS ClimateNormalsCard View

**Files:**
- Create: `ios/wby/wby/Views/ClimateNormalsCard.swift`

**Step 1: Build the card**

This card uses a custom layout (not HalfCard or FullCard) since it needs a graph. Structure:

1. Header: "CLIMATE NORMALS" label with `thermometer.medium` icon, subtitle "1991-2020"
2. Comparison row: current temp vs normal, colored diff badge
3. 12-month temperature chart: filled band between temp_high and temp_low, line for temp_avg, current month highlighted
4. Precipitation bars below the temp chart

Use SwiftUI `Canvas` or `Path` for the chart — keep it simple for v1, can refine later.

Key implementation notes:
- Accept `ClimateNormalsResponse` and optional `currentTemp: Double?`
- Use `.weatherCard()` modifier for consistent styling
- Use the current month from `Calendar.current` to highlight
- Color the diff badge: `.red` for warmer, `.blue` for colder
- Chart x-axis: 12 months (J, F, M, ...), y-axis: auto-scaled to data range

```swift
struct ClimateNormalsCard: View {
    let normals: ClimateNormalsResponse
    let currentTemp: Double?

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            // Header
            Label("CLIMATE NORMALS", systemImage: "thermometer.medium")
                .font(.caption)
                .foregroundStyle(.secondary)

            // Period
            Text(normals.period)
                .font(.subheadline)
                .foregroundStyle(.secondary)

            // Comparison
            if let avg = normals.today.tempAvg {
                HStack(alignment: .firstTextBaseline) {
                    Text("Normal: \(Int(avg.rounded()))°")
                        .font(.title2)
                    if let diff = normals.today.tempDiff {
                        Text(diffLabel(diff))
                            .font(.subheadline)
                            .foregroundStyle(diff > 0 ? .red : .blue)
                    }
                }
            }

            // Temperature chart
            temperatureChart

            // Precipitation bars
            precipitationBars
        }
        .weatherCard()
    }

    // ... chart implementation using Canvas/Path ...
}
```

Leave detailed chart drawing implementation to the implementer — the key shapes are:
- Filled area between `temp_high` and `temp_low` arrays (12 points, interpolate smooth)
- Line for `temp_avg`
- Vertical highlight bar on current month
- Month labels (J F M A M J J A S O N D) on x-axis
- Precipitation as small bars below, scaled independently

**Step 2: Wire into WeatherPageView**

In `WeatherPageView.swift`, add:
- A `@State private var climateNormals: ClimateNormalsResponse?` property
- Fetch in the existing load task: `climateNormals = await weatherService.loadClimateNormalsFromCacheOrFetch(lat: lat, lon: lon)`
- Place `ClimateNormalsCard` in the view body after the existing cards (conditionally shown when data is available)

**Step 3: Build and verify in preview/simulator**

Run: `xcodebuild -project ios/wby/wby.xcodeproj -scheme wby -destination 'generic/platform=iOS Simulator' build`
Expected: BUILD SUCCEEDED

**Step 4: Commit**

```bash
git add ios/wby/wby/Views/ClimateNormalsCard.swift ios/wby/wby/Views/WeatherPageView.swift
git commit -m "ios: add climate normals card with temperature graph"
```

---

### Task 12: End-to-End Verification

**Step 1: Ensure local-dev is running with data**

```bash
cd server && ./scripts/local-dev.sh up
```

**Step 2: Run the import CLI**

```bash
cd server && DATABASE_URL="postgres://..." /usr/local/go/bin/go run ./cmd/import-normals
```

Expected: Log output showing batches imported

**Step 3: Test the endpoint**

```bash
curl "http://localhost:8080/v1/climate-normals?lat=60.17&lon=24.94"
```

Expected: JSON with station, period, today (interpolated), monthly (12 entries)

**Step 4: Run all server tests**

```bash
cd server && /usr/local/go/bin/go test ./...
```

Expected: All pass

**Step 5: Commit any final fixes, then create feature branch commit**

```bash
git add -A
git commit -m "feat: climate normals with interpolated daily values"
```
