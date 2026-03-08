# Climate Normals Feature Design

## Overview

Show FMI 30-year climate normals (1991–2020) in the iOS app, allowing users to compare current conditions against historical averages. Data is served from a dedicated endpoint, fetched once, and cached indefinitely on the client.

## Data Source

FMI WFS stored query: `fmi::observations::weather::monthly::30year::timevaluepair`

Available normal periods: 1971–2000, 1981–2010, 1991–2020. We use 1991–2020.

Key parameters (of 47 available):

| Param | Meaning |
|---|---|
| TAP1M | Mean temperature |
| TAMAXP1M | Mean daily max temperature |
| TAMINP1M | Mean daily min temperature |
| PRAP1M | Precipitation sum (mm) |

Sample testdata saved to `server/internal/fmi/testdata/climate_normals.xml`.

## Database

New table via migration:

```sql
CREATE TABLE climate_normals (
    fmisid      INTEGER NOT NULL REFERENCES stations(fmisid),
    month       SMALLINT NOT NULL CHECK (month BETWEEN 1 AND 12),
    period      TEXT NOT NULL DEFAULT '1991-2020',
    temp_avg    REAL,
    temp_high   REAL,
    temp_low    REAL,
    precip_mm   REAL,
    PRIMARY KEY (fmisid, month, period)
);
```

Data is imported via a one-time CLI command (`go run ./cmd/import-normals`) that queries FMI for all stations in our `stations` table and upserts into `climate_normals`. Can be re-run when a new normal period is published.

## API

### `GET /v1/climate-normals?lat=X&lon=Y`

Finds nearest station via PostGIS, returns 12 monthly values plus an interpolated snapshot for today.

Response:

```json
{
  "station": {
    "name": "Helsinki Kaisaniemi",
    "distance_km": 1.2
  },
  "period": "1991-2020",
  "today": {
    "temp_avg": 2.3,
    "temp_high": 5.1,
    "temp_low": -0.8,
    "precip_mm_day": 1.5,
    "temp_diff": 2.7
  },
  "monthly": [
    { "month": 1, "temp_avg": -3.1, "temp_high": -0.9, "temp_low": -6.5, "precip_mm": 52.0 },
    { "month": 2, "temp_avg": -4.0, "temp_high": -1.5, "temp_low": -7.8, "precip_mm": 36.0 },
    ...
  ]
}
```

- `today.temp_diff` = current observed temperature minus interpolated `temp_avg`. Null if current temp unavailable.
- `today.precip_mm_day` = monthly precip divided by days in month, interpolated to today.
- Server-side interpolation uses cosine blend between two surrounding months (monthly value placed at mid-month, the 15th).

Cache-Control: `public, max-age=86400` (1 day).

## Interpolation

Each monthly normal is placed at the 15th of its month. For a given date, we find the two surrounding mid-month points and cosine-interpolate:

```
t = (date - midMonth_before) / (midMonth_after - midMonth_before)
weight = (1 - cos(t * pi)) / 2
value = value_before * (1 - weight) + value_after * weight
```

This produces a smooth daily curve where early March leans toward February's average and late March toward April's.

## iOS

### Data Layer

- New method on `WeatherService`: `fetchClimateNormals(lat:lon:)` returning a `ClimateNormalsResponse` model
- Disk cache keyed by lat/lon (same 0.01° grid snapping as weather cache)
- Fetched once per location; loaded from cache on subsequent launches

### ClimateNormalsCard

- Header: "Climate Normals" with subtitle "1991–2020"
- Comparison row: current temp vs today's normal, colored diff badge ("+4.2° warmer" / "-2.1° colder")
- Graph: 12-month temperature chart with high/low range as filled band, current month highlighted. Precipitation as bars.
- Loads independently from main weather request; can display from cache instantly.
