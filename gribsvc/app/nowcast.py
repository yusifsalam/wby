"""Radar extrapolation nowcast — precipitation forecast frames from the
observed composites.

Motion is estimated with OpenCV's DIS dense optical flow between the newest
radar frames (on a dBR-scaled image, the rainymotion "Dense" scheme), then the
latest frame is advected backward-semi-Lagrangian to each lead time under a
constant motion field. Output frames are written next to the observations as
``nowcast_rr_<validtime>.tif`` + JSON sidecar in the same uint16/scale/nodata
encoding, so geotiff.py serves them unchanged. Each run overwrites the frames
for its valid times and prunes leftovers from older runs.
"""

import json
import os
from datetime import datetime, timedelta, timezone
from pathlib import Path

import cv2
import numpy as np
from PIL import Image

from . import geotiff
from .config import config
from .grib import GribError

FRAME_STEP = timedelta(minutes=5)
STAMP_FORMAT = "%Y%m%dT%H%MZ"
# 75 min of leads: the newest observed frame (the run origin) trails wall-clock
# time by a few minutes, and clients scrub a full hour ahead of "now".
DEFAULT_LEADS = 15

OBS_PREFIX = "radar_rr_"
NOWCAST_PREFIX = "nowcast_rr_"

NODATA = 65535

# dBR window for the flow-estimation image: 10*log10(mm/h) mapped to uint8.
_DB_MIN = -15.0
_DB_MAX = 25.0


def frame_name(at: datetime) -> str:
    return NOWCAST_PREFIX + at.strftime(STAMP_FORMAT) + ".tif"


def _list_obs_frames(base: Path) -> list[tuple[datetime, Path]]:
    frames = []
    for p in base.glob(OBS_PREFIX + "*.tif"):
        try:
            at = datetime.strptime(p.name[len(OBS_PREFIX) : -len(".tif")], STAMP_FORMAT)
        except ValueError:
            continue
        frames.append((at.replace(tzinfo=timezone.utc), p))
    frames.sort()
    return frames


def _flow_image(values: np.ndarray) -> np.ndarray:
    """Rain rate -> uint8 dBR image for optical flow. Nodata counts as dry."""
    rr = np.nan_to_num(values, nan=0.0)
    db = 10.0 * np.log10(np.maximum(rr, 10 ** (_DB_MIN / 10.0)))
    scaled = (db - _DB_MIN) / (_DB_MAX - _DB_MIN)
    return (np.clip(scaled, 0.0, 1.0) * 255.0).astype(np.uint8)


def _mean_flow(frames: list[tuple[datetime, np.ndarray]]) -> np.ndarray:
    """Average per-step (5 min) motion over adjacent frame pairs.

    Pairs separated by more than one step (a gap in the window) still
    contribute: their displacement is divided by the number of steps spanned.
    """
    dis = cv2.DISOpticalFlow_create(cv2.DISOPTICAL_FLOW_PRESET_MEDIUM)
    flows = []
    for (t0, v0), (t1, v1) in zip(frames, frames[1:]):
        steps = (t1 - t0) / FRAME_STEP
        if steps <= 0:
            continue
        flow = dis.calc(_flow_image(v0), _flow_image(v1), None)
        flows.append(flow / steps)
    if not flows:
        raise GribError("nowcast: no usable frame pair for motion estimation")
    return np.mean(flows, axis=0)


def _advect(values: np.ndarray, flow: np.ndarray, steps: int) -> np.ndarray:
    """Backward semi-Lagrangian: sample the source frame at x - steps*flow."""
    h, w = values.shape
    grid_x, grid_y = np.meshgrid(
        np.arange(w, dtype=np.float32), np.arange(h, dtype=np.float32)
    )
    map_x = grid_x - steps * flow[..., 0]
    map_y = grid_y - steps * flow[..., 1]
    return cv2.remap(
        values.astype(np.float32),
        map_x,
        map_y,
        interpolation=cv2.INTER_LINEAR,
        borderMode=cv2.BORDER_CONSTANT,
        borderValue=float("nan"),
    )


def _write_frame(
    base: Path, valid: datetime, run: datetime, values: np.ndarray, bbox: list
) -> None:
    """Encode mm/h back to the uint16 radar convention, sidecar-first like the
    fetcher so a frame is never visible without metadata."""
    raw = np.round(np.nan_to_num(values, nan=0.0) / 0.01)
    encoded = np.clip(raw, 0, NODATA - 1).astype(np.uint16)
    encoded[np.isnan(values)] = NODATA

    name = frame_name(valid)
    sidecar = {
        "param": "rr",
        "time": valid.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "run": run.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "bbox": bbox,
        "scale": 0.01,
        "nodata": NODATA,
        "units": "mm/h",
    }
    (base / name).with_suffix(".json").write_text(json.dumps(sidecar))

    tmp = base / ("." + name + ".tmp")
    Image.fromarray(encoded).save(tmp, format="TIFF")
    os.replace(tmp, base / name)


def _prune(base: Path, keep: set[str]) -> None:
    for p in base.glob(NOWCAST_PREFIX + "*"):
        if p.name not in keep and p.with_suffix(".tif").name not in keep:
            p.unlink(missing_ok=True)


def run(leads: int = DEFAULT_LEADS, source_frames: int = 3) -> dict:
    """Produce nowcast frames from the newest observed radar frames."""
    base = config.grib_data_dir
    obs = _list_obs_frames(base)
    if len(obs) < 2:
        raise GribError("nowcast: need at least two radar frames")

    loaded = []
    meta = None
    for at, path in obs[-source_frames:]:
        values, meta = geotiff._load(path)
        loaded.append((at, values))

    flow = _mean_flow(loaded)
    run_time, latest = loaded[-1]

    written = []
    keep = set()
    for k in range(1, leads + 1):
        valid = run_time + k * FRAME_STEP
        _write_frame(base, valid, run_time, _advect(latest, flow, k), meta["bbox"])
        written.append(valid.strftime("%Y-%m-%dT%H:%M:%SZ"))
        keep.add(frame_name(valid))
    _prune(base, keep)

    return {
        "run": run_time.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "frames": written,
        "source_frames": len(loaded),
    }
