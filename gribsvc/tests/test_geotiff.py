"""Radar GeoTIFF parsing/extraction tests.

Unlike the GRIB tests these need no external fixture: a synthetic uint16 tiff +
sidecar is generated per test, mirroring what the Go fetcher writes.
"""

import json

import numpy as np
import pytest
from PIL import Image

from app import geotiff
from app.grib import GribError, parse_time

BBOX = [19.0, 59.0, 32.0, 71.0]
TIME = "2026-09-01T16:05:00Z"
NODATA = 65535
ROWS, COLS = 24, 26


@pytest.fixture
def radar_tif(tmp_path):
    """A 24x26 raster: value 150 (=1.5 mm/h) everywhere, nodata in the NE corner."""
    raw = np.full((ROWS, COLS), 150, dtype=np.uint16)
    raw[0, COLS - 1] = NODATA
    raw[ROWS - 1, 0] = 0
    path = tmp_path / "radar_rr_20260901T160500Z.tif"
    Image.fromarray(raw).save(path)
    path.with_suffix(".json").write_text(
        json.dumps(
            {
                "param": "rr",
                "time": TIME,
                "bbox": BBOX,
                "scale": 0.01,
                "nodata": NODATA,
                "units": "mm/h",
            }
        )
    )
    return path


def test_list_params(radar_tif):
    params = geotiff.list_params(radar_tif)
    assert len(params) == 1
    assert params[0]["param"] == "rr"
    assert params[0]["valid_time"] == TIME
    assert params[0]["units"] == "mm/h"


def test_extract_bbox_scales_and_masks(radar_tif):
    out = geotiff.extract_bbox(radar_tif, "rr", tuple(BBOX), step=1, at=None)
    assert out["rows"] == ROWS and out["cols"] == COLS
    assert out["valid_time"] == TIME
    # Row 0 is the northernmost row (GetMap order).
    assert out["lats"][0][0] > out["lats"][-1][0]
    # Values are scaled to mm/h; nodata is null.
    assert out["values"][1][1] == pytest.approx(1.5)
    assert out["values"][0][COLS - 1] is None
    assert out["values"][ROWS - 1][0] == pytest.approx(0.0)


def test_extract_bbox_subsets_and_strides(radar_tif):
    out = geotiff.extract_bbox(radar_tif, "rr", (24.0, 62.0, 28.0, 66.0), step=2, at=None)
    assert 0 < out["rows"] < ROWS
    assert 0 < out["cols"] < COLS
    for row in out["lats"]:
        for lat in row:
            assert 61.0 < lat < 67.0


def test_extract_points(radar_tif):
    out = geotiff.extract_points(radar_tif, "rr", [(60.17, 24.94)], parse_time(TIME))
    pt = out["points"][0]
    assert pt["value"] == pytest.approx(1.5)
    assert abs(pt["grid_lat"] - 60.17) < 1.0


def test_param_and_time_mismatch(radar_tif):
    with pytest.raises(GribError):
        geotiff.extract_bbox(radar_tif, "temperature", tuple(BBOX), step=1, at=None)
    with pytest.raises(GribError):
        geotiff.extract_bbox(
            radar_tif, "rr", tuple(BBOX), step=1, at=parse_time("2026-09-01T17:00:00Z")
        )


def test_missing_sidecar(tmp_path):
    raw = np.zeros((4, 4), dtype=np.uint16)
    path = tmp_path / "orphan.tif"
    Image.fromarray(raw).save(path)
    with pytest.raises(GribError, match="sidecar"):
        geotiff.extract_bbox(path, "rr", (0.0, 0.0, 1.0, 1.0), step=1, at=None)
