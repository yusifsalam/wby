"""GRIB2 parsing — the only module that touches pygrib.

Keeping pygrib confined here means a later swap to cfgrib/xarray (if files need
label-based time/level selection or lazy loading of large GRIBs) stays contained.
"""

from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

import numpy as np
import pygrib


class GribError(Exception):
    """Raised when a file cannot be parsed or a requested field is not found."""


def _iso(dt: Optional[datetime]) -> Optional[str]:
    if dt is None:
        return None
    # pygrib validDate is a naive datetime in UTC.
    return dt.strftime("%Y-%m-%dT%H:%M:%SZ")


def _matches_param(grb, param: str) -> bool:
    p = param.strip().lower()
    return p in (str(grb.shortName).lower(), str(grb.name).lower())


def _matches_time(grb, at: Optional[datetime]) -> bool:
    if at is None:
        return True
    valid = grb.validDate
    if valid is None:
        return False
    return valid.replace(tzinfo=None) == at.replace(tzinfo=None)


def _clean(value) -> Optional[float]:
    """Convert a (possibly masked / NaN) grid value to a JSON-safe float/None."""
    if value is None or value is np.ma.masked:
        return None
    f = float(value)
    if np.isnan(f) or np.isinf(f):
        return None
    return f


def list_params(path: Path) -> list[dict]:
    """List every message in the file with its identifying metadata."""
    out: list[dict] = []
    grbs = pygrib.open(str(path))
    try:
        for grb in grbs:
            out.append(
                {
                    "param": grb.shortName,
                    "name": grb.name,
                    "level": grb.level,
                    "type_of_level": grb.typeOfLevel,
                    "valid_time": _iso(grb.validDate),
                    "units": getattr(grb, "units", None),
                }
            )
    finally:
        grbs.close()
    return out


def _find_message(path: Path, param: str, at: Optional[datetime]):
    """Return the first message matching param (+ optional time). Caller closes."""
    grbs = pygrib.open(str(path))
    try:
        for grb in grbs:
            if _matches_param(grb, param) and _matches_time(grb, at):
                # Read everything we need before closing the file handle.
                return grb, grbs
    except Exception as exc:  # noqa: BLE001 - surface a clean parse error
        grbs.close()
        raise GribError(f"failed to read {path.name}: {exc}") from exc
    grbs.close()
    raise GribError(f"field not found: param={param!r} time={_iso(at)}")


def _normalize_lon(lon: float, lon_grid: np.ndarray) -> float:
    """Shift a query lon into the grid's longitude convention (0..360 vs -180..180)."""
    if lon_grid.max() > 180.0 and lon < 0:
        return lon + 360.0
    if lon_grid.min() < 0 and lon > 180.0:
        return lon - 360.0
    return lon


def extract_points(
    path: Path, param: str, points: list[tuple[float, float]], at: Optional[datetime]
) -> dict:
    """Nearest-gridpoint value lookup for a list of (lat, lon) points."""
    grb, grbs = _find_message(path, param, at)
    try:
        values = grb.values
        lats, lons = grb.latlons()
        results = []
        for lat, lon in points:
            qlon = _normalize_lon(lon, lons)
            d = (lats - lat) ** 2 + (lons - qlon) ** 2
            i, j = np.unravel_index(int(np.argmin(d)), d.shape)
            results.append(
                {
                    "lat": lat,
                    "lon": lon,
                    "value": _clean(values[i, j]),
                    "grid_lat": float(lats[i, j]),
                    "grid_lon": float(lons[i, j]),
                }
            )
        return {
            "param": param,
            "valid_time": _iso(grb.validDate),
            "units": getattr(grb, "units", None),
            "points": results,
        }
    finally:
        grbs.close()


def extract_bbox(
    path: Path,
    param: str,
    bbox: tuple[float, float, float, float],
    step: int,
    at: Optional[datetime],
) -> dict:
    """Subset the field to a bbox (min_lon, min_lat, max_lon, max_lat) as a grid."""
    min_lon, min_lat, max_lon, max_lat = bbox
    grb, grbs = _find_message(path, param, at)
    try:
        data, lats, lons = grb.data(
            lat1=min_lat, lat2=max_lat, lon1=min_lon, lon2=max_lon
        )
        step = max(1, int(step))
        data = np.asarray(data)[::step, ::step]
        lats = np.asarray(lats)[::step, ::step]
        lons = np.asarray(lons)[::step, ::step]

        # Render masked/NaN as null in JSON.
        masked = np.ma.getmaskarray(data) if np.ma.isMaskedArray(data) else np.isnan(data)
        grid = np.where(masked, None, data.astype(object))

        return {
            "param": param,
            "valid_time": _iso(grb.validDate),
            "units": getattr(grb, "units", None),
            "rows": int(data.shape[0]),
            "cols": int(data.shape[1]),
            "lats": lats.tolist(),
            "lons": lons.tolist(),
            "values": grid.tolist(),
        }
    finally:
        grbs.close()


def field_grid(path: Path, param: str, at: Optional[datetime]) -> tuple[np.ndarray, np.ndarray, np.ndarray, dict]:
    """Return (values, lats, lons, meta) for the whole field — used by rendering."""
    grb, grbs = _find_message(path, param, at)
    try:
        values = np.asarray(grb.values, dtype=float)
        lats, lons = grb.latlons()
        meta = {
            "param": param,
            "valid_time": _iso(grb.validDate),
            "units": getattr(grb, "units", None),
        }
        return values, np.asarray(lats), np.asarray(lons), meta
    finally:
        grbs.close()


def parse_time(raw: Optional[str]) -> Optional[datetime]:
    """Parse an RFC3339/ISO time string to a naive UTC datetime (pygrib convention)."""
    if raw is None or raw.strip() == "":
        return None
    s = raw.strip().replace("Z", "+00:00")
    dt = datetime.fromisoformat(s)
    if dt.tzinfo is not None:
        dt = dt.astimezone(timezone.utc).replace(tzinfo=None)
    return dt
