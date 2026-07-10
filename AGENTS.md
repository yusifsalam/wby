# Repository Guidelines

## Project Structure & Module Organization
This repo has three main apps:
- `server/`: Go backend API + data ingestion.
  - `cmd/server/`: server entrypoint.
  - `cmd/import-normals/`: one-off climate normals importer.
  - `internal/api/`: HTTP handlers and JSON response mapping.
  - `internal/config/`: environment config parsing.
  - `internal/fetcher/`: background observation ingestion loop.
  - `internal/fmi/`: FMI client/parsers and XML fixtures in `internal/fmi/testdata/`.
  - `internal/store/`: Postgres/PostGIS persistence.
  - `internal/weather/`: domain models, service logic, caching.
  - `migrations/`: SQL schema migrations.
  - `scripts/local-dev.sh`: local DB/server bootstrap for macOS.
  - `docker-compose.prod.yml`: production overlay attaching `server`/`web` to the shared `edge` network (no host ports).
- `infra/`: host-wide shared stack — Caddy HTTPS ingress + observability (Loki + Alloy + Grafana, `logging` profile); `conf/caddy/`, `conf/{loki,alloy,grafana}/`. See `infra/README.md`.
- `ios/wby/wby/`: SwiftUI iOS app (`Background/`, `Components/`, `Models/`, `Services/`, `Views/`, `ContentView.swift`).
- `ios/wby/config/`: environment-specific `Keys.*.plist` files for API base URL and request signing credentials.
- `gribsvc/`: standalone Python GRIB2 service (FastAPI + pygrib) that serves numeric extraction + PNG tiles; not yet wired into the server.
  - `app/`: service code (`main.py` routes, `grib.py` parsing seam, `render.py`, `sources.py`, `config.py`).
  - `tests/`: pytest suite; `explore.ipynb`/`explore.py`: interactive exploration.
  - `testdata/`: local `.grib2` fixtures (gitignored).
- `docs/plans/`: implementation and design notes.

## Build, Test, and Development Commands
- `cd server && ./scripts/local-dev.sh up`: start local Postgres (if needed), initialize schema, run API.
- `cd server && ./scripts/local-dev.sh init-db`: initialize DB only.
- `cd server && ./scripts/local-dev.sh run-server`: run API only.
- `cd server && go build ./cmd/server`: compile backend binary.
- `cd server && go run ./cmd/import-normals`: import climate normals for known station IDs (requires DB + stations loaded).
- `cd server && go test ./...`: run all backend tests.
- `cd server && go test ./internal/fmi -v`: run FMI parser tests with fixture coverage.
- `cd server && go test ./internal/store -v`: run store tests (requires running Postgres/PostGIS).
- `go run ./server/cmd/server`: direct server run (requires env vars like `DATABASE_URL`).
- `cd server && docker compose up --build`: run DB + gribsvc + server + web via Docker Compose (override publishes them on loopback ports).
- `cd server && docker compose -p wby -f docker-compose.yml -f docker-compose.prod.yml up -d --build`: production deploy — services join the shared `edge` network, no host ports (Caddy in `infra/` is the ingress).
- `cd infra && docker network create edge && docker compose -p infra --profile logging up -d`: bring up the shared Caddy ingress + Loki/Alloy/Grafana log stack (Grafana at `localhost:3000`; needs `GRAFANA_ADMIN_PASSWORD` in `infra/.env`).
- Xcode MCP `BuildProject` (project `ios/wby/wby.xcodeproj`, scheme `wby`): preferred iOS build check.
- `cd gribsvc && pip install -r requirements-dev.txt && pytest`: run GRIB2 service tests (GRIB-fixture tests skip when `testdata/` is empty).
- `cd gribsvc && GRIB_DATA_DIR=./testdata uvicorn app.main:app --port 9090`: run the GRIB2 service locally.

## Coding Style & Naming Conventions
- Go: keep code `gofmt`-clean; package names are lowercase; exported identifiers use `PascalCase`.
- Swift: types use `PascalCase`, properties/functions use `camelCase`; keep one major view per file in `Views/`.
- Match existing API naming: backend JSON is snake_case; Swift models map via `CodingKeys`.
- Prefer small, focused changes; avoid unrelated refactors in the same PR.

## Testing Guidelines
- Backend tests use Go’s standard testing package.
- Test files end with `_test.go`; test funcs follow `TestXxx`.
- Reuse/add fixtures under `server/internal/fmi/testdata/` for parser behavior.
- `server/internal/store` tests are integration-style and expect Postgres/PostGIS to be available.
- For iOS UI changes, keep previews working with mock data and verify in simulator.
- `gribsvc/` uses pytest; keep `app/grib.py` the only pygrib seam, and let GRIB-fixture tests skip gracefully when `testdata/` is empty.

## Commit & Pull Request Guidelines
- Follow existing commit style: scoped, imperative subjects (examples: `server: ...`, `ios: ...`, `tooling: ...`, `feat: ...`, `chore: ...`).
- Split commits by function (for example, backend API vs iOS UI).
- PRs should include:
  - what changed and why,
  - validation performed (commands run),
  - screenshots for UI updates,
  - notes for config/migration impacts.

## Security & Configuration Tips
- Start from `server/.env.example`; do not commit secrets.
- Local development expects Postgres + PostGIS.
- Key backend env vars: `DATABASE_URL`, `PORT`, `FMI_BASE_URL`, `FMI_API_KEY`, `FMI_TIMESERIES_URL`, `CLIENT_SECRETS`, `REQUEST_SIGNATURE_MAX_AGE_SECONDS`.
- Observability env vars (log stack): `GRAFANA_ADMIN_PASSWORD` (required), `GRAFANA_BIND`, `GRAFANA_PORT`, `GRAFANA_ROOT_URL`.
- iOS request signing/base URL live in `ios/wby/config/Keys.Debug.plist` and `ios/wby/config/Keys.Release.plist` (see `*.example.plist` templates). Keep secrets out of git history.
