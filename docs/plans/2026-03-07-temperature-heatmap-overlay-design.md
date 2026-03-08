# Temperature Heatmap Overlay Design (v1 Snapshot)

## Goal

Add a map weather field overlay for the iOS app, starting with a viewport-scoped temperature heatmap snapshot. After v1, extend to a forecast time slider.

## Scope

- In scope (v1):
  - Temperature heatmap overlay only
  - Current snapshot only (no animation)
  - Viewport-driven requests (visible map bbox)
- Out of scope (v1):
  - Time slider/animation
  - Precipitation radar overlay rendering
  - Station network layer rendering

## Context and Data Sources

- Current backend weather API is point-based (`/v1/weather`) and not designed for map overlays.
- FMI stored queries provide grid products suitable for overlays:
  - `fmi::forecast::meps::surface::grid` (recommended primary source)
  - `fmi::forecast::edited::weather::scandinavia::grid` (fallback candidate)
- Query descriptors show support for:
  - `bbox`, `starttime`, `endtime`, `parameters`, `format`
  - Grid outputs via `GridSeriesObservation`
  - `format` including `grib2` and `netcdf` for MEPS surface grid

## Approaches Considered

1. Server raster overlay (recommended)
   - Backend fetches grid, colorizes to transparent PNG, iOS draws map overlay.
   - Pros: simple iOS integration, good performance, consistent rendering.
   - Cons: backend rasterization work.
2. Server vector cells/points
   - Backend returns sampled values, iOS interpolates/colors.
   - Pros: simpler backend output structure.
   - Cons: heavier client rendering, more battery/memory pressure.
3. Client-side grid decoding
   - iOS fetches/decode GRIB/NetCDF directly.
   - Pros: minimal backend logic.
   - Cons: highest complexity and risk on mobile.

Decision: use server raster overlay for v1.

## v1 Architecture

### Backend

- Add endpoint: `GET /v1/map/temperature`
- Inputs:
  - `bbox=minLon,minLat,maxLon,maxLat` (EPSG:4326)
  - `width`, `height` (requested output pixels)
- Processing:
  - Validate/clamp bbox and dimensions
  - Select snapshot time at current UTC hour
  - Query FMI MEPS surface grid with:
    - `storedquery_id=fmi::forecast::meps::surface::grid`
    - `parameters=Temperature`
    - `starttime=endtime=<current hour UTC>`
    - `bbox=<normalized bbox>`
    - `format=netcdf` (or `grib2` if decoder support dictates)
  - Decode grid values and map to a transparent temperature color ramp
  - Render PNG with requested dimensions
- Output:
  - `Content-Type: image/png`
  - `Cache-Control: public, max-age=120`
  - `X-Data-Time: <ISO8601>`
  - `X-Temp-Min: <float>`
  - `X-Temp-Max: <float>`

### iOS

- Add map view with overlay layer support.
- On map camera idle:
  - Compute visible region bbox
  - Request `/v1/map/temperature` with bbox + viewport size
  - Replace overlay with fade transition
- Apply request debouncing (300-500ms) during camera movement.
- Keep last successful overlay visible on refresh failure.

## API Contract

### Request

`GET /v1/map/temperature?bbox=...&width=...&height=...`

- `bbox`: required, 4 comma-separated floats (lon/lat bounds).
- `width`, `height`: required, integers, server-clamped.

### Success Response

- `200 OK`
- Body: PNG bytes (transparent heatmap)
- Headers: `X-Data-Time`, `X-Temp-Min`, `X-Temp-Max`, cache control

### Error Responses

- `400` for invalid params/bounds
- `502` when FMI fetch/transform fails
- JSON error body for non-200 responses

## Validation and Caching

- Server-side in-memory cache key:
  - `hour + rounded bbox + width bucket + height bucket`
- TTL: 1-3 minutes.
- Normalize and bound bbox to practical Finland extent for safety/perf.

## Testing Plan

- Backend unit tests:
  - Param validation
  - Bbox normalization/clamping
  - Color scale bounds and nodata handling
- Backend integration tests:
  - Fixture grid -> deterministic PNG dimensions/alpha coverage
- iOS validation:
  - Overlay aligns with viewport
  - Pan/zoom requests debounce correctly
  - Last-good overlay retained on failures

## Follow-up (v2)

- Add `time` parameter to endpoint for forecast frames.
- Introduce iOS time slider and frame animation pipeline.
- Reuse same endpoint with different timestamps.
