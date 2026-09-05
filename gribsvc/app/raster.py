"""Binary raster responses: a regular lat/lon grid as little-endian float32.

The JSON bbox extract ships every gridpoint's lat, lon and value as text, which
for a full-resolution radar frame is ~19 MB and about a second of encoder CPU
per frame. The Go server only needs the regular grid's extent plus the cell
values, so this module packs those as a flat float32 body (row-major,
north-to-south, west-to-east; NaN = masked) with the geometry in headers.
"""

from typing import Optional

import numpy as np

HEADER_ROWS = "X-Grid-Rows"
HEADER_COLS = "X-Grid-Cols"
HEADER_MIN_LAT = "X-Grid-Min-Lat"
HEADER_MAX_LAT = "X-Grid-Max-Lat"
HEADER_MIN_LON = "X-Grid-Min-Lon"
HEADER_MAX_LON = "X-Grid-Max-Lon"
HEADER_VALID_TIME = "X-Valid-Time"
HEADER_UNITS = "X-Units"
# Series responses: frame count and comma-separated valid times, frames
# concatenated in that order in the body.
HEADER_FRAMES = "X-Grid-Frames"
HEADER_VALID_TIMES = "X-Valid-Times"

MEDIA_TYPE = "application/octet-stream"


def regular_grid(
    values: np.ndarray,
    lats: np.ndarray,
    lons: np.ndarray,
    valid_time: Optional[str],
    units: Optional[str],
) -> dict:
    """Describe a 2-D field on a regular lat/lon lattice.

    ``values`` may be a masked array or carry NaN for missing cells; both come
    out as NaN. Rows are reordered so row 0 is the northernmost, whatever order
    the source used. Extents are the centres of the corner cells, matching the
    JSON extract's lattice.
    """
    if np.ma.isMaskedArray(values):
        values = values.filled(np.nan)
    values = np.asarray(values, dtype=np.float32)
    if values.ndim != 2 or values.shape != np.shape(lats) or values.shape != np.shape(lons):
        raise ValueError("raster: values, lats and lons must share one 2-D shape")

    if values.shape[0] > 1 and lats[0, 0] < lats[-1, 0]:
        values = values[::-1]

    return {
        "rows": int(values.shape[0]),
        "cols": int(values.shape[1]),
        "min_lat": float(np.min(lats)),
        "max_lat": float(np.max(lats)),
        "min_lon": float(np.min(lons)),
        "max_lon": float(np.max(lons)),
        "valid_time": valid_time,
        "units": units,
        "values": np.ascontiguousarray(values),
    }


def encode(grid: dict) -> bytes:
    return grid["values"].astype("<f4", copy=False).tobytes()


def headers(grid: dict) -> dict:
    out = {
        HEADER_ROWS: str(grid["rows"]),
        HEADER_COLS: str(grid["cols"]),
        HEADER_MIN_LAT: repr(grid["min_lat"]),
        HEADER_MAX_LAT: repr(grid["max_lat"]),
        HEADER_MIN_LON: repr(grid["min_lon"]),
        HEADER_MAX_LON: repr(grid["max_lon"]),
    }
    if grid.get("valid_time"):
        out[HEADER_VALID_TIME] = grid["valid_time"]
    if grid.get("units"):
        out[HEADER_UNITS] = str(grid["units"])
    return out


def series(frames: list[dict]) -> dict:
    """Combine per-frame regular_grid() results sharing one lattice into a
    series: a (frames, rows, cols) float32 stack plus the frames' valid times."""
    if not frames:
        raise ValueError("raster: series needs at least one frame")
    first = frames[0]
    for f in frames[1:]:
        if (f["rows"], f["cols"]) != (first["rows"], first["cols"]):
            raise ValueError("raster: series frames must share one grid shape")
    out = {k: first[k] for k in ("rows", "cols", "min_lat", "max_lat", "min_lon", "max_lon", "units")}
    out["valid_times"] = [f["valid_time"] for f in frames]
    out["values"] = np.stack([f["values"] for f in frames])
    return out


def series_headers(grid: dict) -> dict:
    out = headers({**grid, "valid_time": None})
    out[HEADER_FRAMES] = str(len(grid["valid_times"]))
    out[HEADER_VALID_TIMES] = ",".join(t or "" for t in grid["valid_times"])
    return out
