# Weather by Yusif (wby)

Weather app with:
- Go backend (`server/`) for FMI ingestion + weather API
- Astro SSR web frontend (`web/`) for cached city weather pages
- SwiftUI iOS client (`ios/wby/`)
- Python GRIB2 service (`gribsvc/`) that parses GRIB2 into JSON values + PNG tiles (standalone; not yet wired into the server)

## Repository Layout

- `server/cmd/server/`: API entrypoint
- `server/cmd/import-normals/`: one-off climate normals importer
- `server/internal/api/`: HTTP handlers (`/v1/weather`, `/v1/map/temperature`, `/v1/climate-normals`, `/v1/leaderboard`, `/health`)
- `server/internal/config/`: environment configuration loading/parsing
- `server/internal/fetcher/`: background station/observation ingestion loop
- `server/internal/fmi/`: FMI WFS client/parsers + XML fixtures, Timeseries UV client
- `server/internal/store/`: Postgres/PostGIS storage
- `server/internal/weather/`: service/domain/cache logic
- `server/migrations/`: DB schema
- `server/scripts/local-dev.sh`: local macOS bootstrap
- `web/`: Astro SSR frontend; server-side signs requests to the Go API and caches fixed city pages
- `ios/wby/wby/`: app code (`Background`, `Components`, `Models`, `Services`, `Views`)
- `ios/wby/config/`: `Keys.$CONFIGURATION.plist` templates and local key files
- `gribsvc/`: standalone Python GRIB2 service (FastAPI + pygrib); see `gribsvc/README.md`

## Prerequisites

- Go (matching `server/go.mod`)
- Node.js 22.12+ (for Astro)
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
| `CLIENT_SECRETS` | (empty) | Comma-separated `client_id:secret` pairs for `/v1/*` request signing |
| `REQUEST_SIGNATURE_MAX_AGE_SECONDS` | `300` | Allowed timestamp skew for signed requests |

## Web Frontend

The Astro app renders fixed Finnish city pages with server-side calls to the signed Go API. The browser never receives the API signing secret.

```bash
cd web
npm install
WBY_API_CLIENT_ID=web WBY_API_CLIENT_SECRET=dev-secret npm run dev
```

For local development against the Go API, make sure the backend has a matching signing entry:

```bash
cd server
CLIENT_SECRETS=web:dev-secret ./scripts/local-dev.sh up
```

Useful web commands:

```bash
cd web
npm test
npm run check
npm run build
```

Import climate normals after stations are loaded:

```bash
cd server
go run ./cmd/import-normals
```

## Docker Compose (Optional)

```bash
cd server
docker compose up --build
```

Set `CLIENT_SECRETS` and `WBY_API_CLIENT_SECRET` to matching values, for example `CLIENT_SECRETS=web:dev-secret` and `WBY_API_CLIENT_SECRET=dev-secret`.

Locally, the auto-merged `docker-compose.override.yml` publishes the services on loopback ports (server `8080`, web `4321`, db `5432`, gribsvc `9091`). In **production** the base stack publishes no host ports; deploy with the prod overlay so `server`/`web` join the shared `edge` network for Caddy:

```bash
cd server
docker compose -p wby -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

## Shared infrastructure (Caddy + Observability)

Host-wide services — the **Caddy** HTTPS ingress and the observability stack (**Alloy** ships every container's logs to **Loki**, visualized by **Grafana**) — live in the top-level [`infra/`](infra/) stack, separate from any app so the VPS can host multiple apps behind one ingress. Apps and Caddy meet on a shared external Docker network, `edge`.

```bash
cd infra
docker network create edge          # one-time
cp .env.example .env                # set GRAFANA_ADMIN_PASSWORD
docker compose -p infra up -d caddy                 # ingress only
docker compose -p infra --profile logging up -d     # ingress + logging
```

Grafana is at http://localhost:3000 locally, and behind Caddy at `https://logs.yourweatherapp.fi` in production (set `GRAFANA_ROOT_URL` accordingly). The "Logs Overview" dashboard is host-wide — use the **Project** selector to scope to an app. Explore logs with LogQL, e.g. `{compose_project="wby"} | json` or `{service="server", level="ERROR"}`. Caddy emits JSON access logs (`{compose_project="infra", service="caddy"} | json`), skipping the `/health` probe. See [`infra/README.md`](infra/README.md) for the app-onboarding contract.

## iOS App

- Open `ios/wby/wby.xcodeproj`
- Run scheme `wby`
- Simulator uses `http://localhost:8080` by default (`WeatherService.resolveBaseURL`)
- The app reads `API_BASE_URL`, `API_CLIENT_ID`, and `API_CLIENT_SECRET` from a bundled `Keys.plist`
  generated from `ios/wby/config/Keys.$CONFIGURATION.plist`

If missing, create from templates:

```bash
cp ios/wby/config/Keys.Debug.example.plist ios/wby/config/Keys.Debug.plist
cp ios/wby/config/Keys.Release.example.plist ios/wby/config/Keys.Release.plist
```

## API

Available routes:
- `GET /v1/weather?lat=<float>&lon=<float>`
- `GET /v1/map/temperature?bbox=<minLon,minLat,maxLon,maxLat>&width=<int>&height=<int>` (PNG)
- `GET /v1/climate-normals?lat=<float>&lon=<float>&current_temp=<float optional>`
- `GET /v1/leaderboard?lat=<float>&lon=<float>&timeframe=now`

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

Store integration tests (requires running Postgres/PostGIS):

```bash
go test ./internal/store -v
```

iOS build check:
- Use Xcode MCP `BuildProject` on the `wby` scheme/project.

## Notes

- Weather data from Finnish Meteorological Institute (FMI): observations and forecasts via the public WFS API (`opendata.fmi.fi`), UV forecasts via the Timeseries API (`data.fmi.fi`, requires API key).
- The server continuously refreshes station observations in the background.
- UV forecast data is merged into hourly and daily forecasts at request time. When no API key is configured, UV fields are omitted gracefully.
