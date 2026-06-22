"""Render tests.

The synthetic-grid test needs no GRIB fixture and always runs. The end-to-end
render test goes through the HTTP API and skips when no fixture is present.
"""

import io

import numpy as np
from PIL import Image

from app import render


def _synthetic_grid():
    lat_axis = np.linspace(71.0, 59.0, 60)   # descending, like many GRIB grids
    lon_axis = np.linspace(19.0, 32.0, 70)   # ascending
    lons, lats = np.meshgrid(lon_axis, lat_axis)
    # A smooth field so the colormap has a gradient to show.
    values = lats + lons
    return values, lats, lons


def test_render_png_dimensions_and_format():
    values, lats, lons = _synthetic_grid()
    png = render.render_png(
        values, lats, lons,
        bbox=(19.0, 59.0, 32.0, 71.0),
        width=128, height=96, colormap="jet",
    )
    assert png[:8] == b"\x89PNG\r\n\x1a\n"  # PNG magic
    img = Image.open(io.BytesIO(png))
    assert img.size == (128, 96)
    assert img.mode == "RGBA"


def test_render_marks_missing_transparent():
    values, lats, lons = _synthetic_grid()
    values[:10, :] = np.nan  # a band of missing data
    png = render.render_png(
        values, lats, lons,
        bbox=(19.0, 59.0, 32.0, 71.0),
        width=64, height=64, colormap="precip",
    )
    img = Image.open(io.BytesIO(png)).convert("RGBA")
    alpha = np.asarray(img)[:, :, 3]
    # The top band (max latitude) maps to NaN -> fully transparent pixels exist.
    assert (alpha == 0).any()
    # And some pixels are opaque where data is valid.
    assert (alpha == 255).any()


def test_render_endpoint(sample_grib, first_param):
    from fastapi.testclient import TestClient

    from app.main import app

    client = TestClient(app)
    resp = client.post(
        "/grib/render",
        json={
            "file": sample_grib.name,
            "param": first_param,
            "bbox": {"min_lon": 19, "min_lat": 59, "max_lon": 32, "max_lat": 71},
            "width": 128,
            "height": 128,
        },
    )
    assert resp.status_code == 200, resp.text
    assert resp.headers["content-type"] == "image/png"
    assert resp.content[:8] == b"\x89PNG\r\n\x1a\n"
