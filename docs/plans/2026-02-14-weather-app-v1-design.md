# Weather App v1 Design

## Overview

A Finland-focused weather app for iOS, powered by FMI Open Data. A Go caching server sits between the iOS client and FMI's WFS API, ensuring clients never hit FMI directly.

## Architecture

```
┌─────────────┐         ┌──────────────────────────────┐        ┌───────────┐
│             │  JSON    │          Go Server            │  WFS   │           │
│   iOS App   │◄────────►│                              │◄──────►│  FMI API  │
│  (SwiftUI)  │  REST    │  ┌────────┐  ┌────────────┐ │  XML   │           │
│             │         │  │In-mem  │  │ Background │ │        └───────────┘
└─────────────┘         │  │cache   │  │ fetcher    │ │
                        │  └───┬────┘  └─────┬──────┘ │
                        │      │             │         │
                        │      ▼             ▼         │
                        │  ┌─────────────────────┐     │
                        │  │ PostgreSQL + PostGIS │     │
                        │  └─────────────────────┘     │
                        └──────────────────────────────┘
                        ┌──────────────────────────────┐
                        │     VPS (Hetzner / DO)       │
                        └──────────────────────────────┘
```

### Components

- **iOS app** — SwiftUI (iOS 26+), GPS location, Apple Weather-inspired UI. Talks to our server over REST/JSON.
- **Go server** — two jobs: (1) background fetcher that pulls FMI data on a schedule into Postgres, (2) REST API that serves cached data to clients with an in-process cache for hot paths.
- **PostgreSQL + PostGIS** — durable store for observations and forecasts. PostGIS for spatial queries as needs grow.
- **FMI WFS API** — only the server talks to FMI. Clients never touch it.

## FMI Data Sources

### Stored queries used in v1

- `fmi::observations::weather::timevaluepair` — current conditions from weather stations
- `fmi::forecast::harmonie::surface::point::timevaluepair` — high-res Harmonie forecast for arbitrary coordinates

### FMI constraints

- Default count limit: 20,000 (time steps x locations x parameters)
- Rate limits: not documented, to be determined empirically
- Data format: GML/XML via WFS

## Backend

### Background Fetcher

**Observations (current conditions):**
- Runs on a schedule (interval TBD — needs empirical testing of FMI update cadence)
- Fetches observations for all Finnish weather stations in a single WFS call
- Parses XML timevaluepair response: temperature, wind speed/direction, humidity, pressure, weather symbol
- Upserts into Postgres with station ID, timestamp, and geometry (station lat/lon)

**Forecasts (daily):**
- On-demand with caching — Harmonie point forecasts are for arbitrary coordinates, so we cannot prefetch everything
- When a client requests a forecast for a GPS coordinate:
  1. Round coordinate to a grid cell (~1km resolution)
  2. Check Postgres for a fresh forecast for that grid cell
  3. If stale or missing, fetch from FMI, parse, store, and serve
  4. In-process cache sits in front of Postgres for hot grid cells

### REST API

Single endpoint for v1:

```
GET /v1/weather?lat=60.17&lon=24.94
```

Response:

```json
{
  "station": { "name": "Helsinki Kaisaniemi", "distance_km": 1.2 },
  "current": {
    "temperature": -3.2,
    "feels_like": -7.1,
    "wind_speed": 4.5,
    "wind_direction": 220,
    "humidity": 85,
    "pressure": 1013.2,
    "symbol": "cloudy",
    "observed_at": "2026-02-14T10:00:00Z"
  },
  "daily_forecast": [
    {
      "date": "2026-02-14",
      "high": -1.0,
      "low": -8.5,
      "symbol": "partly_cloudy",
      "wind_speed_avg": 3.2,
      "precipitation_mm": 0.5
    }
  ]
}
```

One request gets everything needed to render the main screen. The server stitches together the nearest station observation + the grid-cell forecast.

## iOS App

### Target

- iOS 26+ minimum deployment target
- SwiftUI only

### Screen

Single scrollable view (Apple Weather-inspired, not a clone):

1. **Header** — location name (reverse-geocoded via CoreLocation), current temperature, weather symbol, "feels like"
2. **Current conditions card** — wind, humidity, pressure in a grid layout
3. **Daily forecast list** — 7+ days, each row: day name, weather symbol, high/low with temperature range bar

### Tech Stack

- SwiftUI — entire UI
- CoreLocation — GPS coordinate + reverse geocoding for place name
- URLSession — networking (one endpoint, no third-party deps needed)
- Swift concurrency (async/await) — network calls and location updates
- No third-party dependencies

### Data Flow

```
App launch
  → Request location permission
  → Get GPS coordinate
  → GET /v1/weather?lat=...&lon=...
  → Decode JSON into Swift models
  → Render UI
  → Pull-to-refresh to reload
```

### Offline Behavior

Cache last successful response to disk (Codable JSON file in caches directory). On network failure, show stale data with "Last updated X minutes ago" indicator.

## Deployment & Infrastructure

### VPS

- Single VPS (Hetzner or DigitalOcean), 2 vCPU / 2-4GB RAM
- Docker Compose: Go server + PostgreSQL/PostGIS
- Caddy reverse proxy — automatic HTTPS via Let's Encrypt
- Custom domain for API (e.g., api.yourweatherapp.fi)

### Deployment Flow

- Go server builds into static binary inside Docker image
- Push to container registry (GHCR or Docker Hub)
- `docker compose pull && docker compose up -d` on VPS
- Automate later with GitHub Actions

### Monitoring

- `/health` endpoint on Go server
- Uptime check via free service (UptimeRobot or similar)
- Structured JSON logging

## Scope

### In scope (v1)

- iOS app (iOS 26+, SwiftUI, GPS-only location)
- Go caching server with background FMI fetcher + REST API
- PostgreSQL + PostGIS on single VPS with Docker Compose
- Current conditions from nearest FMI weather station
- Daily forecast from Harmonie model, on-demand with caching
- Single-screen UI, Apple Weather-inspired
- Offline fallback with cached last response
- HTTPS via Caddy

### Out of scope (future versions)

- Hourly forecast
- Weather alerts/warnings
- City search / multiple saved locations
- watchOS, macOS, iPadOS, visionOS
- Home screen / Lock Screen widgets
- Multiple data sources (OpenWeatherMap, Yr.no, etc.)
- Redis/Valkey caching layer
- Radar imagery
- Push notifications
- User accounts / authentication
- App Store submission
