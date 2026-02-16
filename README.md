# Weather by Yusif (wby)

Weather app with:
- Go backend (`server/`) for FMI ingestion + weather API
- SwiftUI iOS client (`ios/wby/`)

## Repository Layout

- `server/cmd/server/`: API entrypoint
- `server/internal/api/`: HTTP handlers (`/v1/weather`, `/health`)
- `server/internal/fmi/`: FMI WFS client/parsers + XML fixtures, Timeseries UV client
- `server/internal/store/`: Postgres/PostGIS storage
- `server/internal/weather/`: service/domain/cache logic
- `server/migrations/`: DB schema
- `server/scripts/local-dev.sh`: local macOS bootstrap
- `ios/wby/wby/`: app code (`Models`, `Services`, `Views`)

## Prerequisites

- Go (matching `server/go.mod`)
- PostgreSQL + PostGIS (local)
- Xcode (for iOS)
- macOS with Homebrew (recommended for local DB flow)

## Local Backend Quick Start (No Docker)

```bash
cd server
./scripts/local-dev.sh up
```

The script will:
1. Start local PostgreSQL if needed (`brew services`)
2. Create DB role/database (defaults: `wby` / `wby`)
3. Ensure PostGIS extension exists
4. Apply all migrations from `migrations/`
5. Run API on `:8080`

Useful variants:

```bash
./scripts/local-dev.sh init-db
./scripts/local-dev.sh run-server
```

The script sources `server/.env` if present. Env vars you can override:

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_NAME` | `wby` | Postgres database name |
| `DB_USER` | `wby` | Postgres role |
| `DB_PASSWORD` | `wby` | Postgres password |
| `PORT` | `8080` | Server listen port |
| `DATABASE_URL` | (derived) | Full Postgres connection string |
| `FMI_BASE_URL` | `https://opendata.fmi.fi/wfs` | FMI WFS endpoint |
| `FMI_API_KEY` | (empty) | FMI API key for `data.fmi.fi` (enables UV forecasts) |
| `FMI_TIMESERIES_URL` | `https://data.fmi.fi` | FMI Timeseries API base URL |

## iOS App

- Open `ios/wby/wby.xcodeproj`
- Run scheme `wby`
- Simulator uses `http://localhost:8080` by default (`WeatherService.resolveBaseURL`)

Optional override: set `API_BASE_URL` in app `Info.plist`.

## API

```bash
curl "http://localhost:8080/v1/weather?lat=60.1699&lon=24.9384"
```

Response includes:
- `station`
- `current`
- `hourly_forecast`
- `daily_forecast`

Health check:

```bash
curl http://localhost:8080/health
```

## Testing

Run backend tests:

```bash
cd server
go test ./...
```

Focused parser tests:

```bash
go test ./internal/fmi -v
```

## Notes

- Weather data from Finnish Meteorological Institute (FMI): observations and forecasts via the public WFS API (`opendata.fmi.fi`), UV forecasts via the Timeseries API (`data.fmi.fi`, requires API key).
- The server continuously refreshes station observations in the background.
- UV forecast data is merged into hourly and daily forecasts at request time. When no API key is configured, UV fields are omitted gracefully.
