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

# Observability log stack (Loki + Alloy + Grafana), gated behind the "logging" profile.
# Grafana at http://localhost:3000; needs GRAFANA_ADMIN_PASSWORD set.
cd server && docker compose -p wby \
  -f docker-compose.yml -f docker-compose.override.yml -f docker-compose.observability.yml \
  --profile logging up -d

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

### Observability

Optional log stack in `server/docker-compose.observability.yml`, gated behind the
`logging` compose profile so it never starts with the core stack:

- **Alloy** (`conf/alloy/config.alloy`) discovers every container via the Docker
  socket and ships logs to Loki, promoting the slog `level` field to a label.
- **Loki** (`conf/loki/loki-config.yaml`) — single-binary, filesystem storage,
  90-day retention.
- **Grafana** (`conf/grafana/provisioning/`) — auto-provisions the Loki
  datasource + a starter dashboard; served at `logs.yourweatherapp.fi` behind
  Caddy (`GRAFANA_ROOT_URL`), or `localhost:3000` directly.
- Caddy emits JSON access logs to stdout (skipping `/health`), captured by Alloy.

Gotcha: passing explicit `-f` files disables Compose's auto-merge of
`docker-compose.override.yml` (which supplies the arm64 DB image locally), so
list it explicitly when running the full stack.

### iOS Architecture

- `WeatherService` (actor): REST client with offline JSON cache fallback
- `LocationService` (@Observable): CoreLocation GPS + reverse geocoding + altitude
- `ContentView`: main scrollable weather screen; individual cards in `Views/`
- `SmartSymbol`: maps FMI weather symbol codes (1-99, 100+ for night) to SF Symbols
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

## FMI Data Sources

### WFS (opendata.fmi.fi)

Three stored queries used via the public WFS endpoint:

| Query | Purpose |
|-------|---------|
| `fmi::observations::weather::timevaluepair` | All Finnish station observations (bbox 19,59,32,71) |
| `fmi::observations::radiation::timevaluepair` | Radiation data (merged by station/time) |
| `fmi::forecast::edited::weather::scandinavia::point::timevaluepair` | Point forecasts (Harmonie model, 11-day window) |

Radiation observations come from a separate query and are merged into the nearest station's data as a fallback when the primary station lacks a radiometer.

### Timeseries (data.fmi.fi)

UV forecast data is fetched from the Smartmet Timeseries API at `data.fmi.fi` using `producer=uv`. Requires `FMI_API_KEY` env var. When no API key is configured, UV data is gracefully skipped.
