# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Environment

- **Go binary**: `/usr/local/go/bin/go` (not in default PATH)
- **Go module name**: `wby`
- **Go version**: 1.26.0
- **Swift**: 6.0, targeting iOS 26+
- **Database**: PostgreSQL 18 + PostGIS 3.6

## Build & Test Commands

```bash
# Go server
cd server && /usr/local/go/bin/go build ./cmd/server
cd server && /usr/local/go/bin/go test ./...                  # all tests
cd server && /usr/local/go/bin/go test ./internal/fmi -v       # parser tests (uses testdata/)
cd server && /usr/local/go/bin/go test ./internal/weather -v   # cache/service tests
cd server && /usr/local/go/bin/go test ./internal/store -v     # integration tests (requires DB)
cd server && /usr/local/go/bin/go test -run TestParseFoo ./internal/fmi  # single test

# Local dev (starts Postgres via Homebrew, applies migrations, runs server on :8080)
cd server && ./scripts/local-dev.sh up

# Production deploy: services join the shared `edge` network, no host ports.
cd server && docker compose -p wby -f docker-compose.yml -f docker-compose.prod.yml up -d --build

# Shared infra stack (Caddy ingress + Loki/Alloy/Grafana, "logging" profile).
# Grafana at http://localhost:3000; needs GRAFANA_ADMIN_PASSWORD in infra/.env.
docker network create edge   # one-time
cd infra && docker compose -p infra --profile logging up -d

# iOS (use Xcode MCP tools, NOT xcodebuild CLI)
# Build:  mcp__xcode__BuildProject (scheme: "wby")
# Tests:  mcp__xcode__RunAllTests / mcp__xcode__RunSomeTests
# Issues: mcp__xcode__XcodeListNavigatorIssues
```

No linter is configured. Code should be `gofmt`-clean.

## Architecture

Three apps: a Go backend (`server/`), a SwiftUI iOS client (`ios/wby/`), and a
standalone Python GRIB2 service (`gribsvc/`, FastAPI + pygrib) that parses GRIB2
into JSON values + PNG tiles — not yet wired into the server. See
`gribsvc/README.md`.

### Server Data Flow

1. **Background fetcher** (`internal/fetcher/`) polls FMI every 10 minutes, bulk-upserts stations and observations into Postgres.
2. **API request** hits `GET /v1/weather?lat=X&lon=Y` (`internal/api/handler.go`).
3. **Service** (`internal/weather/service.go`) orchestrates the response:
   - Finds nearest station via PostGIS spatial query (`<->` operator)
   - Returns latest observation from that station
   - Snaps coordinates to 0.01° grid (~1km) for forecast cache keys
   - Checks 3-tier cache: in-process (10min TTL) → Postgres (3h TTL) → FMI API
   - Combines current conditions + hourly forecast (12h) + daily forecast (7-10d)
4. **FMI client** (`internal/fmi/client.go`) fetches WFS XML; **parser** (`internal/fmi/parser.go`) converts XML to domain models.
5. **Store** (`internal/store/store.go`) handles all Postgres/PostGIS persistence with `pgx/v5` batch operations.

### Shared infrastructure (`infra/`)

Host-wide Caddy ingress + observability live in the top-level `infra/` stack
(Compose project `infra`), separate from the Weather app so the VPS can host
multiple apps behind one ingress. Apps and Caddy meet on an external Docker
network, `edge` (created once with `docker network create edge`); databases stay
private per app.

- **Caddy** (`infra/conf/caddy/`) — the only stack that publishes ports 80/443.
  A base `Caddyfile` imports per-app fragments from `sites/`; `weather.caddy`
  routes `yourweatherapp.fi`→`weather-web:4321`, `api.…/v1/*`+`/health`→
  `weather-api:8080` (other API paths 404), `logs.…`→`grafana:3000`. Upstreams
  are the `edge` aliases set in `server/docker-compose.prod.yml`.
- **Observability** (`logging` profile) — **Alloy** (`conf/alloy/config.alloy`)
  discovers every container via the Docker socket and ships logs to **Loki**
  (`conf/loki/`, filesystem, 90-day retention), promoting the slog `level` field
  to a label. **Grafana** (`conf/grafana/provisioning/`) auto-provisions the Loki
  datasource + a host-wide "Logs Overview" dashboard (a `$project` selector
  filters by `compose_project`); served at `logs.yourweatherapp.fi` behind Caddy
  (`GRAFANA_ROOT_URL`), or `localhost:3000` directly.

Weather's own stack (`server/docker-compose.yml`) is db + gribsvc + server + web
only. The base publishes no host ports: local overlays
(`docker-compose.override.yml` / `.dev.yml`) publish them on loopback and supply
the arm64 DB image; the production overlay (`docker-compose.prod.yml`) joins
`edge` with no host ports. Explicit `-f` files disable Compose's auto-merge of
the override, so `.dev.yml` is self-contained.

### iOS Architecture

- `WeatherService` (actor): REST client with offline JSON cache fallback
- `LocationService` (@Observable): CoreLocation GPS + reverse geocoding + altitude
- `ContentView`: main scrollable weather screen; individual cards in `Views/`
- `SmartSymbol`: maps FMI SmartSymbol codes (1-77, 100+ for night) to SF Symbols
- No third-party dependencies

## Key Conventions

- **Commit style**: scoped imperative subjects — `server: ...`, `ios: ...`, `feat: ...`, `fix: ...`
- **JSON contract**: backend uses snake_case; Swift maps via `CodingKeys`
- **Error wrapping**: `fmt.Errorf("context: %w", err)`
- **Logging**: `log/slog` with JSON output
- **Nullable numerics**: FMI returns NaN for missing values; Go models use `*float64`, Swift uses optionals
- **One view per file** in `ios/wby/wby/Views/`

## Database

Migrations in `server/migrations/` (numbered SQL files, applied sequentially by `local-dev.sh`). Key tables:

- `stations` — FMISID primary key, PostGIS `geography` column with GIST index
- `observations` — foreign key to stations, timestamped weather parameters + `extra` JSONB
- `forecasts` — keyed by grid lat/lon + date, 20+ forecast parameter columns
- `hourly_forecasts` — keyed by grid lat/lon + forecast time
- `climate_normals` — FMI's published monthly 1991–2020 normals (`cmd/import-normals`), served by `/v1/climate-normals`
- `daily_climate_normals` — per calendar day (month+day, incl. 29 Feb): temperature avg/high/low, feels-like avg/high/low (wind chill from hourly temp+wind), wind avg, daily max gust, humidity, precipitation mm, wet-day share (≥0.1 mm), snow depth, plus 24-hour UTC curves for temp/feels-like/wind/humidity. Computed from station history by `cmd/import-daily-normals` (only Kaisaniemi so far; hourly params exist there from ~2000, hourly precip only from 2012 so precip stays daily), served by `/v1/climate-normals/daily`, which also returns a `precipitation` block: observed mm so far today and month-to-date (hourly `PRA_PT1H_ACC` from FMI since the 1st, cached per station-hour) against the daily normals summed over the same days. The app prefers the daily card and falls back to the monthly one only where no daily normals exist.

## FMI Data Sources

### WFS (opendata.fmi.fi)

Three stored queries used via the public WFS endpoint:

| Query | Purpose |
|-------|---------|
| `fmi::observations::weather::timevaluepair` | All Finnish station observations (bbox 19,59,32,71) |
| `fmi::observations::radiation::timevaluepair` | Radiation data (merged by station/time) |
| `fmi::forecast::edited::weather::scandinavia::point::timevaluepair` | Point forecasts (Harmonie model, 11-day window). Requests an explicit `parameters` list (`forecastParameters` in `client.go`): FMI's defaults plus `SmartSymbol` |

The `symbol` field on hourly/daily forecasts is FMI's **SmartSymbol** code (1–77;
`+100` = night variant, assigned by FMI), the same scheme the official FMI app
uses. The daily symbol is the 15:00 local-time hour. `weather_symbol3_mode`
carries the older `WeatherSymbol3` code for reference only.

Radiation observations come from a separate query and are merged into the nearest station's data as a fallback when the primary station lacks a radiometer.

### Open radar composites (openwms.fmi.fi)

Keyless radar rain-rate composites (`Radar:suomi_rr_eureffin`, 5-min cadence)
are fetched as EPSG:4326 **GeoTIFF data grids** via GetMap (`RADAR_FETCH_ENABLE=1`).
The fetcher keeps a rolling ~70min window of frames (uint16, value÷100 = mm/h,
65535 = outside coverage) plus JSON sidecars in the gribsvc volume; gribsvc
parses them next to the GRIBs. After new frames land the server POSTs gribsvc
`/nowcast/run`, which extrapolates the newest frames (OpenCV DIS optical flow
+ backward semi-Lagrangian advection, `gribsvc/app/nowcast.py`) into
`nowcast_rr_*` frames out to +75min in the same encoding.
`/v1/map/precipitation/observed` serves past scrubber frames and
`/v1/map/precipitation/nowcast` future ones; clients render both client-side
like the 12h forecast, falling back to the API-key WMS tiles only on a miss.

### Timeseries (data.fmi.fi)

UV forecast data is fetched from the Smartmet Timeseries API at `data.fmi.fi` using `producer=uv`. Requires `FMI_API_KEY` env var. When no API key is configured, UV data is gracefully skipped.
