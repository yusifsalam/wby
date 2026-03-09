# Repository Guidelines

## Project Structure & Module Organization
This repo has two main apps:
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
- `ios/wby/wby/`: SwiftUI iOS app (`Background/`, `Components/`, `Models/`, `Services/`, `Views/`, `ContentView.swift`).
- `ios/wby/config/`: environment-specific `Keys.*.plist` files for API base URL and request signing credentials.
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
- `cd server && docker compose up --build`: run DB + server + Caddy via Docker Compose.
- Xcode MCP `BuildProject` (project `ios/wby/wby.xcodeproj`, scheme `wby`): preferred iOS build check.

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
- iOS request signing/base URL live in `ios/wby/config/Keys.Debug.plist` and `ios/wby/config/Keys.Release.plist` (see `*.example.plist` templates). Keep secrets out of git history.
