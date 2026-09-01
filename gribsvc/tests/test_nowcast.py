"""Nowcast extrapolation tests on a synthetic moving rain blob.

A Gaussian blob drifting east at a constant pixel speed is written as three
observed frames; the nowcast should keep it moving at that speed.
"""

import json
from datetime import datetime, timezone

import numpy as np
import pytest
from PIL import Image

from app import nowcast
from app.grib import GribError

BBOX = [19.0, 59.0, 32.0, 71.5]
SIZE = 128
SPEED = 4  # px east per 5-min frame
NODATA = 65535


def _blob_frame(cx: float, cy: float) -> np.ndarray:
    y, x = np.mgrid[0:SIZE, 0:SIZE]
    rr = 5.0 * np.exp(-((x - cx) ** 2 + (y - cy) ** 2) / (2 * 12.0**2))
    return rr


def _write_obs(base, at: datetime, rr: np.ndarray) -> None:
    raw = np.round(rr / 0.01).astype(np.uint16)
    raw[:, :2] = NODATA
    name = "radar_rr_" + at.strftime("%Y%m%dT%H%MZ")
    Image.fromarray(raw).save(base / (name + ".tif"))
    (base / (name + ".json")).write_text(
        json.dumps(
            {
                "param": "rr",
                "time": at.strftime("%Y-%m-%dT%H:%M:%SZ"),
                "bbox": BBOX,
                "scale": 0.01,
                "nodata": NODATA,
                "units": "mm/h",
            }
        )
    )


@pytest.fixture
def data_dir(tmp_path, monkeypatch):
    monkeypatch.setenv("GRIB_DATA_DIR", str(tmp_path))
    times = [
        datetime(2026, 9, 1, 16, 0, tzinfo=timezone.utc),
        datetime(2026, 9, 1, 16, 5, tzinfo=timezone.utc),
        datetime(2026, 9, 1, 16, 10, tzinfo=timezone.utc),
    ]
    for i, at in enumerate(times):
        _write_obs(tmp_path, at, _blob_frame(40 + i * SPEED, 64))
    return tmp_path


def _peak(path) -> tuple[int, int]:
    raw = np.array(Image.open(path)).astype(float)
    raw[raw == NODATA] = 0
    return np.unravel_index(int(np.argmax(raw)), raw.shape)


def test_run_extrapolates_motion(data_dir):
    out = nowcast.run(leads=3)
    assert out["run"] == "2026-09-01T16:10:00Z"
    assert out["frames"] == [
        "2026-09-01T16:15:00Z",
        "2026-09-01T16:20:00Z",
        "2026-09-01T16:25:00Z",
    ]

    # Blob center was at x=48 in the latest frame; each lead adds ~SPEED px.
    for k in (1, 2, 3):
        path = data_dir / f"nowcast_rr_20260901T1{610 + 5 * k}Z.tif"
        assert path.is_file()
        py, px = _peak(path)
        assert py == pytest.approx(64, abs=3)
        assert px == pytest.approx(48 + k * SPEED, abs=3)


def test_sidecar_written(data_dir):
    nowcast.run(leads=1)
    meta = json.loads((data_dir / "nowcast_rr_20260901T1615Z.json").read_text())
    assert meta["param"] == "rr"
    assert meta["time"] == "2026-09-01T16:15:00Z"
    assert meta["run"] == "2026-09-01T16:10:00Z"
    assert meta["bbox"] == BBOX
    assert meta["scale"] == 0.01


def test_nodata_preserved(data_dir):
    nowcast.run(leads=1)
    raw = np.array(Image.open(data_dir / "nowcast_rr_20260901T1615Z.tif"))
    assert (raw[:, 0] == NODATA).all()


def test_prune_stale_frames(data_dir):
    (data_dir / "nowcast_rr_20260901T1555Z.tif").touch()
    (data_dir / "nowcast_rr_20260901T1555Z.json").touch()
    nowcast.run(leads=1)
    assert not (data_dir / "nowcast_rr_20260901T1555Z.tif").exists()
    assert not (data_dir / "nowcast_rr_20260901T1555Z.json").exists()
    assert (data_dir / "nowcast_rr_20260901T1615Z.tif").exists()


def test_requires_two_frames(tmp_path, monkeypatch):
    monkeypatch.setenv("GRIB_DATA_DIR", str(tmp_path))
    _write_obs(tmp_path, datetime(2026, 9, 1, 16, 0, tzinfo=timezone.utc), _blob_frame(40, 64))
    with pytest.raises(GribError, match="two radar frames"):
        nowcast.run(leads=1)
