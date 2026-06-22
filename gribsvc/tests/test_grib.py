"""GRIB parsing/extraction tests.

These need a real `.grib2` fixture in `testdata/` and auto-skip when none is
present (see conftest.py).
"""

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


def test_parse_time_roundtrip():
    assert grib.parse_time(None) is None
    assert grib.parse_time("") is None
    dt = grib.parse_time("2026-06-22T12:00:00Z")
    assert dt is not None and dt.tzinfo is None
    assert (dt.year, dt.month, dt.hour) == (2026, 6, 12)
