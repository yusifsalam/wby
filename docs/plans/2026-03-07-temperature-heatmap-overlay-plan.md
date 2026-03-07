# Temperature Heatmap Overlay Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a viewport-scoped, current-snapshot temperature heatmap overlay on the iOS map using a new backend raster endpoint.

**Architecture:** The server adds `GET /v1/map/temperature` that fetches current MEPS grid temperature data from FMI, renders a transparent PNG heatmap for the requested bbox and pixel size, and returns image bytes with metadata headers. iOS map camera changes request this endpoint for the visible region and render the returned image as an overlay while keeping the last successful overlay on transient failures.

**Tech Stack:** Go (`net/http`, image/png, FMI WFS client), PostGIS-free map endpoint path, SwiftUI + MapKit (`MKMapView` overlay renderer), existing signed API client.

---

### Task 1: Add map API contract + request validation

**Files:**
- Create: `server/internal/api/map_temperature.go`
- Create: `server/internal/api/map_temperature_test.go`
- Modify: `server/internal/api/handler.go`
- Modify: `server/internal/weather/service.go`

**Step 1: Write the failing tests**

```go
func TestParseMapTemperatureRequest_Valid(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature?bbox=24.7,60.1,25.2,60.4&width=390&height=844", nil)
	got, err := parseMapTemperatureRequest(req)
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	if got.Width != 390 || got.Height != 844 { t.Fatalf("unexpected size: %+v", got) }
}

func TestParseMapTemperatureRequest_InvalidBBox(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature?bbox=bad&width=390&height=844", nil)
	_, err := parseMapTemperatureRequest(req)
	if err == nil { t.Fatal("expected error") }
}
```

**Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/api -run TestParseMapTemperatureRequest -v`  
Expected: FAIL (`parseMapTemperatureRequest` undefined).

**Step 3: Write minimal implementation**

```go
type mapTemperatureRequest struct {
	MinLon, MinLat float64
	MaxLon, MaxLat float64
	Width, Height  int
}

func parseMapTemperatureRequest(r *http.Request) (mapTemperatureRequest, error) {
	// parse bbox + width/height; validate ordering and ranges
}
```

Also in `handler.go`:

```go
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/map/temperature", h.getTemperatureOverlay)
}
```

**Step 4: Run test to verify it passes**

Run: `cd server && go test ./internal/api -run TestParseMapTemperatureRequest -v`  
Expected: PASS.

**Step 5: Commit**

```bash
git add server/internal/api/map_temperature.go server/internal/api/map_temperature_test.go server/internal/api/handler.go server/internal/weather/service.go
git commit -m "server: add map temperature request validation and route stub"
```

### Task 2: Implement FMI temperature grid snapshot fetch

**Files:**
- Create: `server/internal/fmi/grid.go`
- Create: `server/internal/fmi/grid_test.go`
- Create: `server/internal/fmi/testdata/meps_grid_descriptor.xml`
- Modify: `server/internal/fmi/client.go`

**Step 1: Write the failing tests**

```go
func TestBuildMEPSGridQuery(t *testing.T) {
	q := buildMEPSGridQuery(24.7, 60.1, 25.2, 60.4, time.Date(2026,3,7,12,0,0,0,time.UTC))
	if !strings.Contains(q, "storedquery_id=fmi::forecast::meps::surface::grid") { t.Fatal("missing storedquery_id") }
	if !strings.Contains(q, "parameters=Temperature") { t.Fatal("missing Temperature parameter") }
}
```

**Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/fmi -run TestBuildMEPSGridQuery -v`  
Expected: FAIL (`buildMEPSGridQuery` undefined).

**Step 3: Write minimal implementation**

```go
type GridFetcher interface {
	FetchTemperatureGrid(ctx context.Context, bbox [4]float64, at time.Time) ([]byte, error)
}

func buildMEPSGridQuery(minLon, minLat, maxLon, maxLat float64, at time.Time) string {
	// request WFS query for fmi::forecast::meps::surface::grid with bbox + Temperature + netcdf
}
```

Wire method in `client.go`:

```go
func (c *Client) FetchTemperatureGrid(ctx context.Context, bbox [4]float64, at time.Time) ([]byte, error) {
	// perform WFS fetch and return dataset bytes to rasterizer
}
```

**Step 4: Run test to verify it passes**

Run: `cd server && go test ./internal/fmi -run TestBuildMEPSGridQuery -v`  
Expected: PASS.

**Step 5: Commit**

```bash
git add server/internal/fmi/grid.go server/internal/fmi/grid_test.go server/internal/fmi/testdata/meps_grid_descriptor.xml server/internal/fmi/client.go
git commit -m "server: add FMI MEPS temperature grid fetcher"
```

### Task 3: Add temperature rasterizer (grid -> PNG)

**Files:**
- Create: `server/internal/weather/overlay.go`
- Create: `server/internal/weather/overlay_test.go`
- Create: `server/internal/weather/testdata/temperature-grid.json`

**Step 1: Write the failing tests**

```go
func TestRenderTemperatureOverlay_PNGSize(t *testing.T) {
	grid := TemperatureGrid{
		Width: 3, Height: 2,
		Values: []float64{-5, 0, 5, 10, 15, 20},
	}
	pngBytes, meta, err := RenderTemperatureOverlay(grid, 300, 200)
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	if len(pngBytes) == 0 { t.Fatal("expected image bytes") }
	if meta.MinTemp != -5 || meta.MaxTemp != 20 { t.Fatalf("unexpected meta: %+v", meta) }
}
```

**Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/weather -run TestRenderTemperatureOverlay -v`  
Expected: FAIL (`RenderTemperatureOverlay` undefined).

**Step 3: Write minimal implementation**

```go
type OverlayMeta struct {
	DataTime time.Time
	MinTemp  float64
	MaxTemp  float64
}

func RenderTemperatureOverlay(grid TemperatureGrid, width, height int) ([]byte, OverlayMeta, error) {
	// nearest-neighbor resample + blue-to-red color ramp + transparent nodata
}
```

**Step 4: Run test to verify it passes**

Run: `cd server && go test ./internal/weather -run TestRenderTemperatureOverlay -v`  
Expected: PASS.

**Step 5: Commit**

```bash
git add server/internal/weather/overlay.go server/internal/weather/overlay_test.go server/internal/weather/testdata/temperature-grid.json
git commit -m "server: add temperature heatmap rasterizer"
```

### Task 4: Implement `/v1/map/temperature` endpoint end-to-end

**Files:**
- Modify: `server/internal/api/map_temperature.go`
- Create: `server/internal/api/map_temperature_handler_test.go`
- Modify: `server/internal/weather/service.go`

**Step 1: Write the failing handler tests**

```go
func TestGetTemperatureOverlay_OK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature?bbox=24.7,60.1,25.2,60.4&width=390&height=844", nil)
	rr := httptest.NewRecorder()
	h := newTestHandlerWithOverlayStub()
	h.getTemperatureOverlay(rr, req)
	if rr.Code != http.StatusOK { t.Fatalf("expected 200, got %d", rr.Code) }
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" { t.Fatalf("unexpected ct: %s", ct) }
}
```

**Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/api -run TestGetTemperatureOverlay_OK -v`  
Expected: FAIL (stub/service path missing).

**Step 3: Write minimal implementation**

In service:

```go
type TemperatureOverlay struct {
	PNG  []byte
	Meta OverlayMeta
}

func (s *Service) GetTemperatureOverlay(ctx context.Context, req MapOverlayRequest) (*TemperatureOverlay, error) {
	// fetch grid snapshot + render PNG
}
```

In handler:

```go
w.Header().Set("Content-Type", "image/png")
w.Header().Set("Cache-Control", "public, max-age=120")
w.Header().Set("X-Data-Time", overlay.Meta.DataTime.UTC().Format(time.RFC3339))
w.Header().Set("X-Temp-Min", strconv.FormatFloat(overlay.Meta.MinTemp, 'f', 2, 64))
w.Header().Set("X-Temp-Max", strconv.FormatFloat(overlay.Meta.MaxTemp, 'f', 2, 64))
w.Write(overlay.PNG)
```

**Step 4: Run test to verify it passes**

Run: `cd server && go test ./internal/api -run TestGetTemperatureOverlay -v`  
Expected: PASS.

**Step 5: Commit**

```bash
git add server/internal/api/map_temperature.go server/internal/api/map_temperature_handler_test.go server/internal/weather/service.go
git commit -m "server: add temperature overlay map endpoint"
```

### Task 5: Add iOS overlay client models + fetching

**Files:**
- Create: `ios/wby/wby/Models/MapOverlay.swift`
- Modify: `ios/wby/wby/Services/WeatherService.swift`
- Create: `ios/wby/wby/Services/MapOverlayService.swift`

**Step 1: Write failing Swift tests or compile-check snippet**

```swift
struct MapBBox: Equatable {
    let minLon: Double
    let minLat: Double
    let maxLon: Double
    let maxLat: Double
}
```

`MapOverlayService` API:

```swift
func fetchTemperatureOverlay(bbox: MapBBox, width: Int, height: Int) async throws -> TemperatureOverlayImage
```

**Step 2: Run build to verify missing symbols**

Run: `xcodebuild -project ios/wby/wby.xcodeproj -scheme wby -destination 'generic/platform=iOS Simulator' build`  
Expected: FAIL until new files are implemented.

**Step 3: Write minimal implementation**

```swift
struct TemperatureOverlayImage {
    let imageData: Data
    let dataTime: Date?
    let minTemp: Double?
    let maxTemp: Double?
    let bbox: MapBBox
}
```

Implement signed GET to `/v1/map/temperature` and parse metadata headers.

**Step 4: Run build to verify it passes**

Run: `xcodebuild -project ios/wby/wby.xcodeproj -scheme wby -destination 'generic/platform=iOS Simulator' build`  
Expected: PASS.

**Step 5: Commit**

```bash
git add ios/wby/wby/Models/MapOverlay.swift ios/wby/wby/Services/WeatherService.swift ios/wby/wby/Services/MapOverlayService.swift
git commit -m "ios: add temperature overlay client service"
```

### Task 6: Add map screen + overlay rendering

**Files:**
- Create: `ios/wby/wby/Views/WeatherMapView.swift`
- Modify: `ios/wby/wby/ContentView.swift`
- Modify: `ios/wby/wby/Views/WeatherPageView.swift` (if map entry point is placed from weather page)

**Step 1: Write the failing UI wiring change**

Add entry action (toolbar button) and placeholder map view in `ContentView`.

```swift
@State private var showingMap = false
```

```swift
ToolbarItem(placement: .topBarTrailing) {
    Button { showingMap = true } label: { Image(systemName: "map") }
}
```

**Step 2: Build to verify missing view/service references**

Run: `xcodebuild -project ios/wby/wby.xcodeproj -scheme wby -destination 'generic/platform=iOS Simulator' build`  
Expected: FAIL until map view + overlay renderer are fully wired.

**Step 3: Write minimal implementation**

- `WeatherMapView` owns map camera state and computes visible bbox.
- Debounce camera idle updates (300-500ms).
- Request overlay image and draw via `MKMapView` `MKOverlayRenderer`.
- Keep last successful overlay if request fails.

**Step 4: Build to verify it passes**

Run: `xcodebuild -project ios/wby/wby.xcodeproj -scheme wby -destination 'generic/platform=iOS Simulator' build`  
Expected: PASS.

**Step 5: Commit**

```bash
git add ios/wby/wby/Views/WeatherMapView.swift ios/wby/wby/ContentView.swift ios/wby/wby/Views/WeatherPageView.swift
git commit -m "ios: add map view with temperature heatmap overlay"
```

### Task 7: Final verification + docs update

**Files:**
- Modify: `docs/plans/2026-03-07-temperature-heatmap-overlay-design.md` (add implemented notes only if needed)

**Step 1: Run full backend tests**

Run: `cd server && go test ./...`  
Expected: PASS.

**Step 2: Run iOS build check**

Run: `xcodebuild -project ios/wby/wby.xcodeproj -scheme wby -destination 'generic/platform=iOS Simulator' build`  
Expected: PASS.

**Step 3: Manual verification checklist**

- Open map screen.
- Pan/zoom map.
- Confirm heatmap requests follow viewport.
- Confirm overlay fades in and remains on transient network failure.
- Confirm metadata-driven legend text (if included) shows valid min/max.

**Step 4: Commit verification/docs touch-ups**

```bash
git add docs/plans/2026-03-07-temperature-heatmap-overlay-design.md
git commit -m "docs: record temperature overlay implementation notes"
```

**Step 5: Prepare review**

Run:

```bash
git log --oneline --decorate -n 10
git status
```

Expected: clean working tree with task-scoped commits.
