"""Radar GeoTIFF parsing — uint16 rasters fetched from FMI's open radar WMS.

Each ``.tif`` is a single field on a regular lat/lon grid (the fetcher requests
EPSG:4326 GetMap, so GeoServer has already reprojected). Georeferencing and
units come from a JSON sidecar written next to the tiff by the fetcher::

    {"param": "rr", "time": "2026-09-01T16:05:00Z",
     "bbox": [19.0, 59.0, 32.0, 71.5], "scale": 0.01, "nodata": 65535,
     "units": "mm/h"}

Values are ``raw * scale`` with ``nodata`` cells emitted as null, so consumers
get final units and never see the uint16 encoding.
"""

import json
from datetime import datetime
from pathlib import Path
from typing import Optional

import numpy as np
from PIL import Image

from . import raster
from .grib import GribError


def is_geotiff(path: Path) -> bool:
    return path.suffix.lower() in (".tif", ".tiff")


def _sidecar(path: Path) -> dict:
    meta_path = path.with_suffix(".json")
    if not meta_path.is_file():
        raise GribError(f"missing sidecar metadata: {meta_path.name}")
    try:
        meta = json.loads(meta_path.read_text())
    except ValueError as exc:
        raise GribError(f"invalid sidecar metadata {meta_path.name}: {exc}") from exc
    if "bbox" not in meta or len(meta["bbox"]) != 4:
        raise GribError(f"sidecar {meta_path.name} lacks a 4-element bbox")
    return meta


def _load(path: Path) -> tuple[np.ndarray, dict]:
    meta = _sidecar(path)
    try:
        with Image.open(path) as im:
            raw = np.array(im)
    except Exception as exc:  # noqa: BLE001 - surface a clean parse error
        raise GribError(f"failed to read {path.name}: {exc}") from exc
    if raw.ndim != 2:
        raise GribError(f"{path.name}: expected a single-band raster, got shape {raw.shape}")

    scale = float(meta.get("scale", 1.0))
    nodata = meta.get("nodata")

    values = raw.astype(float) * scale
    if nodata is not None:
        values[raw == nodata] = np.nan
    return values, meta


def _matches_time(meta: dict, at: Optional[datetime]) -> bool:
    if at is None:
        return True
    raw = meta.get("time")
    if not raw:
        return False
    parsed = datetime.fromisoformat(raw.replace("Z", "+00:00")).replace(tzinfo=None)
    return parsed == at.replace(tzinfo=None)


def _check_field(path: Path, meta: dict, param: str, at: Optional[datetime]) -> None:
    want = param.strip().lower()
    have = str(meta.get("param", "")).strip().lower()
    if have and want != have:
        raise GribError(f"field not found: param={param!r} (file holds {have!r})")
    if not _matches_time(meta, at):
        raise GribError(f"field not found: param={param!r} time mismatch in {path.name}")


def _latlon_mesh(meta: dict, shape: tuple[int, int]) -> tuple[np.ndarray, np.ndarray]:
    """Cell-center coordinates. Row 0 is the northernmost row (GetMap order)."""
    min_lon, min_lat, max_lon, max_lat = (float(v) for v in meta["bbox"])
    rows, cols = shape
    dy = (max_lat - min_lat) / rows
    dx = (max_lon - min_lon) / cols
    lats = max_lat - (np.arange(rows) + 0.5) * dy
    lons = min_lon + (np.arange(cols) + 0.5) * dx
    return np.meshgrid(lats, lons, indexing="ij")


def list_params(path: Path) -> list[dict]:
    """Describe the raster like grib.list_params describes GRIB messages."""
    values, meta = _load(path)
    return [
        {
            "param": meta.get("param"),
            "name": meta.get("param"),
            "level": 0,
            "type_of_level": "surface",
            "valid_time": meta.get("time"),
            "units": meta.get("units"),
        }
    ]


def extract_points(
    path: Path, param: str, points: list[tuple[float, float]], at: Optional[datetime]
) -> dict:
    """Nearest-gridpoint value lookup, mirroring grib.extract_points."""
    values, meta = _load(path)
    _check_field(path, meta, param, at)
    lats, lons = _latlon_mesh(meta, values.shape)
    results = []
    for lat, lon in points:
        d = (lats - lat) ** 2 + (lons - lon) ** 2
        i, j = np.unravel_index(int(np.argmin(d)), d.shape)
        v = values[i, j]
        results.append(
            {
                "lat": lat,
                "lon": lon,
                "value": None if np.isnan(v) else float(v),
                "grid_lat": float(lats[i, j]),
                "grid_lon": float(lons[i, j]),
            }
        )
    return {
        "param": param,
        "valid_time": meta.get("time"),
        "units": meta.get("units"),
        "points": results,
    }


def _subset(
    path: Path,
    param: str,
    bbox: tuple[float, float, float, float],
    step: int,
    at: Optional[datetime],
) -> tuple[np.ndarray, np.ndarray, np.ndarray, dict]:
    """Load the raster and cut it (and its lat/lon lattice) to the bbox."""
    min_lon, min_lat, max_lon, max_lat = bbox
    values, meta = _load(path)
    _check_field(path, meta, param, at)
    lats, lons = _latlon_mesh(meta, values.shape)

    mask = (
        (lats >= min_lat) & (lats <= max_lat) & (lons >= min_lon) & (lons <= max_lon)
    )
    row_idx = np.where(mask.any(axis=1))[0]
    col_idx = np.where(mask.any(axis=0))[0]
    if row_idx.size == 0 or col_idx.size == 0:
        raise GribError("bbox does not intersect the raster")

    step = max(1, int(step))
    rs = slice(row_idx[0], row_idx[-1] + 1, step)
    cs = slice(col_idx[0], col_idx[-1] + 1, step)
    return values[rs, cs], lats[rs, cs], lons[rs, cs], meta


def extract_bbox(
    path: Path,
    param: str,
    bbox: tuple[float, float, float, float],
    step: int,
    at: Optional[datetime],
) -> dict:
    """Subset the raster to a bbox, mirroring grib.extract_bbox's response."""
    sub, sub_lats, sub_lons, meta = _subset(path, param, bbox, step, at)
    grid = np.where(np.isnan(sub), None, sub.astype(object))
    return {
        "param": param,
        "valid_time": meta.get("time"),
        "units": meta.get("units"),
        "rows": int(sub.shape[0]),
        "cols": int(sub.shape[1]),
        "lats": sub_lats.tolist(),
        "lons": sub_lons.tolist(),
        "values": grid.tolist(),
    }


def extract_bbox_raster(
    path: Path,
    param: str,
    bbox: tuple[float, float, float, float],
    step: int,
    at: Optional[datetime],
) -> dict:
    """Subset the raster to a bbox as a binary-ready regular grid (see raster.py)."""
    sub, sub_lats, sub_lons, meta = _subset(path, param, bbox, step, at)
    return raster.regular_grid(sub, sub_lats, sub_lons, meta.get("time"), meta.get("units"))


def field_grid(path: Path, param: str, at: Optional[datetime]):
    """Return (values, lats, lons, meta) for the whole raster — used by rendering."""
    values, meta = _load(path)
    _check_field(path, meta, param, at)
    lats, lons = _latlon_mesh(meta, values.shape)
    out_meta = {
        "param": param,
        "valid_time": meta.get("time"),
        "units": meta.get("units"),
    }
    return values, lats, lons, out_meta
