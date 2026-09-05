"""GRIB parsing/extraction tests.

These need a real `.grib2` fixture in `testdata/` and auto-skip when none is
present (see conftest.py).
"""

import pytest

from app import grib


def test_list_params(sample_grib):
    params = grib.list_params(sample_grib)
    assert isinstance(params, list)
    assert params, "expected at least one GRIB message"
    first = params[0]
    assert first["param"]
    assert "valid_time" in first


def test_extract_points(sample_grib, first_param):
    # A point near Helsinki; nearest-gridpoint should resolve inside any
    # Finland-covering grid, but we only assert structure so the test stays
    # fixture-agnostic.
    out = grib.extract_points(sample_grib, first_param, [(60.17, 24.94)], None)
    assert out["param"] == first_param
    assert len(out["points"]) == 1
    pt = out["points"][0]
    assert "value" in pt
    assert "grid_lat" in pt and "grid_lon" in pt
    # value is either a float or None (missing/masked), never NaN
    assert pt["value"] is None or isinstance(pt["value"], float)


def test_extract_bbox(sample_grib, first_param):
    out = grib.extract_bbox(
        sample_grib, first_param, (19.0, 59.0, 32.0, 71.0), step=4, at=None
    )
    assert out["rows"] == len(out["values"])
    if out["rows"]:
        assert out["cols"] == len(out["values"][0])
        assert len(out["lats"]) == out["rows"]


def test_extract_bbox_series(sample_grib, first_param):
    times = [
        grib.parse_time(m["valid_time"])
        for m in grib.list_params(sample_grib)
        if m["param"] == first_param and m["valid_time"]
    ]
    if len(times) < 2:
        pytest.skip("sample GRIB has fewer than 2 frames for the first param")
    wanted = sorted(set(times))[:2]

    out = grib.extract_bbox_series(
        sample_grib, first_param, (19.0, 59.0, 32.0, 71.0), step=4, times=wanted
    )
    assert len(out["frames"]) == len(wanted)
    assert len(out["lats"]) == out["rows"]
    assert [grib.parse_time(f["valid_time"]) for f in out["frames"]] == wanted
    for frame in out["frames"]:
        assert len(frame["values"]) == out["rows"]
        assert len(frame["values"][0]) == out["cols"]

    # A one-frame extract of the same hour must agree with the series frame.
    single = grib.extract_bbox(
        sample_grib, first_param, (19.0, 59.0, 32.0, 71.0), step=4, at=wanted[0]
    )
    assert single["values"] == out["frames"][0]["values"]


def test_extract_bbox_series_no_match(sample_grib, first_param):
    with pytest.raises(grib.GribError):
        grib.extract_bbox_series(
            sample_grib, "nosuchparam", (19.0, 59.0, 32.0, 71.0), step=4, times=None
        )


def test_parse_time_roundtrip():
    assert grib.parse_time(None) is None
    assert grib.parse_time("") is None
    dt = grib.parse_time("2026-06-22T12:00:00Z")
    assert dt is not None and dt.tzinfo is None
    assert (dt.year, dt.month, dt.hour) == (2026, 6, 12)


def test_extract_bbox_raster_matches_json(sample_grib, first_param):
    import numpy as np

    from app import grib

    bbox = (19.0, 59.0, 32.0, 71.0)
    out = grib.extract_bbox(sample_grib, first_param, bbox, step=4, at=None)
    grid = grib.extract_bbox_raster(sample_grib, first_param, bbox, step=4, at=None)
    assert (grid["rows"], grid["cols"]) == (out["rows"], out["cols"])
    assert grid["valid_time"] == out["valid_time"]
    assert grid["max_lat"] == pytest.approx(max(out["lats"][0][0], out["lats"][-1][0]))
    # Whatever order pygrib emits, row 0 of the raster is the northern row.
    north_first = out["lats"][0][0] >= out["lats"][-1][0]
    json_north_row = out["values"][0] if north_first else out["values"][-1]
    for j, v in enumerate(json_north_row):
        if v is None:
            assert np.isnan(grid["values"][0, j])
        else:
            assert grid["values"][0, j] == pytest.approx(v, rel=1e-6)
