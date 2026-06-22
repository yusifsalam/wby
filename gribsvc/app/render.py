"""Render a GRIB field to a colormapped, transparent PNG tile.

The output is sampled in EPSG:3857 (Web Mercator) so a tile lines up with a
MapKit / web-mercator basemap when this later feeds a map overlay.

v1 assumes the field is on a regular geographic (lat/lon) grid — true for the
edited/point forecast products we start with. Sampling is nearest-neighbour
along each axis, which is fast and dependency-free (no scipy).
"""

import io
import math
from typing import Optional

import numpy as np
from PIL import Image

_EARTH_RADIUS = 6378137.0

# Small built-in colormaps as RGB stops, expanded to a 256-entry LUT on demand.
_COLORMAP_STOPS = {
    "jet": [(0, 0, 131), (0, 60, 255), (0, 255, 255), (0, 255, 0),
            (255, 255, 0), (255, 60, 0), (131, 0, 0)],
    "precip": [(200, 220, 255), (100, 170, 255), (40, 120, 240),
               (30, 200, 120), (235, 215, 60), (240, 130, 40), (200, 30, 30)],
    "gray": [(0, 0, 0), (255, 255, 255)],
}


def _lon_to_merc_x(lon: float) -> float:
    return math.radians(lon) * _EARTH_RADIUS


def _lat_to_merc_y(lat: float) -> float:
    lat = max(min(lat, 85.05112878), -85.05112878)
    return math.log(math.tan(math.pi / 4 + math.radians(lat) / 2)) * _EARTH_RADIUS


def _merc_x_to_lon(x: np.ndarray) -> np.ndarray:
    return np.degrees(x / _EARTH_RADIUS)


def _merc_y_to_lat(y: np.ndarray) -> np.ndarray:
    return np.degrees(2 * np.arctan(np.exp(y / _EARTH_RADIUS)) - math.pi / 2)


def _build_lut(name: str) -> np.ndarray:
    stops = _COLORMAP_STOPS.get(name, _COLORMAP_STOPS["jet"])
    stops = np.asarray(stops, dtype=float)
    xs = np.linspace(0.0, 1.0, len(stops))
    grid = np.linspace(0.0, 1.0, 256)
    lut = np.empty((256, 3), dtype=np.uint8)
    for c in range(3):
        lut[:, c] = np.interp(grid, xs, stops[:, c]).astype(np.uint8)
    return lut


def _nearest_on_axis(axis: np.ndarray, targets: np.ndarray) -> np.ndarray:
    """Index of the nearest axis value for each target. Handles descending axes."""
    ascending = axis[-1] >= axis[0]
    a = axis if ascending else axis[::-1]
    idx = np.searchsorted(a, targets)
    idx = np.clip(idx, 1, len(a) - 1)
    left = a[idx - 1]
    right = a[idx]
    idx = np.where(targets - left < right - targets, idx - 1, idx)
    if not ascending:
        idx = len(a) - 1 - idx
    return idx.astype(int)


def render_png(
    values: np.ndarray,
    lats: np.ndarray,
    lons: np.ndarray,
    bbox: tuple[float, float, float, float],
    width: int,
    height: int,
    colormap: str = "jet",
    vmin: Optional[float] = None,
    vmax: Optional[float] = None,
) -> bytes:
    min_lon, min_lat, max_lon, max_lat = bbox

    # 1D axes from the (regular) 2D coordinate grids.
    lat_axis = lats[:, 0]
    lon_axis = lons[0, :]

    # Target pixel centres in Web Mercator, then back to lon/lat.
    x0, x1 = _lon_to_merc_x(min_lon), _lon_to_merc_x(max_lon)
    y0, y1 = _lat_to_merc_y(min_lat), _lat_to_merc_y(max_lat)
    px = np.linspace(x0, x1, width, endpoint=False) + (x1 - x0) / (2 * width)
    # Rows go top (max lat) -> bottom (min lat) in image space.
    py = np.linspace(y1, y0, height, endpoint=False) - (y1 - y0) / (2 * height)
    target_lon = _merc_x_to_lon(px)
    target_lat = _merc_y_to_lat(py)

    rows = _nearest_on_axis(lat_axis, target_lat)
    cols = _nearest_on_axis(lon_axis, target_lon)
    sample = values[np.ix_(rows, cols)]

    valid = np.isfinite(sample)
    finite_vals = sample[valid]
    if vmin is None:
        vmin = float(finite_vals.min()) if finite_vals.size else 0.0
    if vmax is None:
        vmax = float(finite_vals.max()) if finite_vals.size else 1.0
    span = vmax - vmin if vmax > vmin else 1.0

    # Replace missing values with vmin before the cast so NaN never reaches
    # astype(uint8) (which would warn); transparency is applied via alpha below.
    safe = np.where(valid, sample, vmin)
    norm = np.clip((safe - vmin) / span, 0.0, 1.0)
    idx = (norm * 255).astype(np.uint8)

    lut = _build_lut(colormap)
    rgb = lut[idx]
    alpha = np.where(valid, 255, 0).astype(np.uint8)
    rgba = np.dstack([rgb, alpha]).astype(np.uint8)

    buf = io.BytesIO()
    Image.fromarray(rgba, mode="RGBA").save(buf, format="PNG")
    return buf.getvalue()
