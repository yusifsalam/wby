# gribsvc

A small Python service that parses GRIB2 files and serves the results over
HTTP/JSON. It does two things:

- **extract** numeric values at points or over a bounding box (JSON)
- **render** a colormapped, transparent PNG tile of a field (image/png)

Phase 1 serves **local files** out of `GRIB_DATA_DIR`. Phase 2 will add fetching
GRIB from FMI open data (the seam is in `app/sources.py`). Parsing uses
[`pygrib`](https://github.com/jswhit/pygrib) and is confined to `app/grib.py`.

This service is standalone for now — wiring it into the Go server (`wby`) and
into docker-compose is a separate, later pass.

## Layout

```
app/
  main.py        FastAPI app + routes
  grib.py        pygrib parsing (the only pygrib seam)
  render.py      numpy array -> colormapped PNG (Pillow)
  sources.py     file resolution (local now, FMI fetch later)
  config.py      env: GRIB_DATA_DIR, PORT
explore.ipynb    interactive notebook: inspect a file, extract, render, map overlay
explore.py       same flow as a cell-script (# %% cells)
testdata/        local .grib2 fixtures (gitignored; drop your own in)
tests/
```

## Run locally

```bash
cd gribsvc
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
GRIB_DATA_DIR=./testdata uvicorn app.main:app --port 9090
```

## Endpoints

| Method | Path             | Returns     |
|--------|------------------|-------------|
| GET    | `/health`        | `{"status":"ok"}` |
| GET    | `/grib/datasets` | files + their parameters/times |
| POST   | `/grib/extract`  | JSON values |
| POST   | `/grib/extract_series` | one bbox grid per requested hour, in one file pass |
| POST   | `/grib/extract_raster` | one bbox grid as binary float32 (`X-Grid-*` headers carry the geometry) |
| POST   | `/grib/extract_raster_series` | `extract_series` as binary: frames concatenated in `X-Valid-Times` order |
| POST   | `/grib/render`   | `image/png` |

### Examples

```bash
curl localhost:9090/health
curl localhost:9090/grib/datasets

# point extraction
curl -XPOST localhost:9090/grib/extract -H 'content-type: application/json' -d '{
  "file": "sample.grib2",
  "param": "2t",
  "points": [{"lat": 60.17, "lon": 24.94}]
}'

# bbox extraction (subsampled every 4th gridpoint)
curl -XPOST localhost:9090/grib/extract -H 'content-type: application/json' -d '{
  "file": "sample.grib2",
  "param": "2t",
  "bbox": {"min_lon": 19, "min_lat": 59, "max_lon": 32, "max_lat": 71},
  "step": 4
}'

# bbox extraction of several hours in one pass (cache warming)
curl -XPOST localhost:9090/grib/extract_series -H 'content-type: application/json' -d '{
  "file": "sample.grib2",
  "param": "2t",
  "bbox": {"min_lon": 19, "min_lat": 59, "max_lon": 32, "max_lat": 71},
  "step": 4,
  "times": ["2026-09-01T12:00:00Z", "2026-09-01T13:00:00Z"]
}'

# bbox extraction as a binary raster: little-endian float32 cells, NaN = masked,
# row-major north-to-south; rows/cols/extent/valid time in X-Grid-* / X-Valid-Time
# headers. What the Go server uses for radar, nowcast and GRIB grids.
curl -XPOST localhost:9090/grib/extract_raster -H 'content-type: application/json' -d '{
  "file": "radar_rr_20260901T160500Z.tif",
  "param": "rr",
  "bbox": {"min_lon": 19, "min_lat": 59, "max_lon": 32, "max_lat": 71.5}
}' -D - --output frame.f32

# rendered tile
curl -XPOST localhost:9090/grib/render -H 'content-type: application/json' -d '{
  "file": "sample.grib2",
  "param": "2t",
  "bbox": {"min_lon": 19, "min_lat": 59, "max_lon": 32, "max_lat": 71},
  "width": 512, "height": 512, "colormap": "jet"
}' --output tile.png
```

`param` matches either the GRIB `shortName` (e.g. `2t`, `tp`) or the full
`name`. `time` is optional (RFC3339); when omitted the first matching message is
used. Colormaps: `jet`, `precip`, `gray`.

## Tests

Drop a representative `.grib2` into `testdata/`, then:

```bash
cd gribsvc
pip install -r requirements-dev.txt
pytest
```

Tests that need a GRIB file auto-skip when `testdata/` has none; the render
test runs on a synthetic grid and always executes.

## Interactive exploration

`explore.ipynb` walks through a file end to end: list datasets, inspect a
message with pygrib, extract point/bbox values, render a tile, and overlay the
field on a map of Finland (coastlines + borders). Needs the extra dev deps:

```bash
pip install -r requirements-dev.txt   # adds matplotlib + cartopy
```

Open it in Jupyter/VS Code/PyCharm with the venv as the kernel, started from
`gribsvc/` so `from app import ...` resolves. `explore.py` is the same flow as a
`# %%` cell-script if you prefer running it in an editor instead of a notebook.

## Docker

```bash
docker build -t gribsvc gribsvc/
docker run --rm -p 9090:9090 -v "$PWD/gribsvc/testdata:/data" gribsvc
```
