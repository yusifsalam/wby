"""FastAPI app exposing GRIB2 extraction and rendering over HTTP/JSON.

Endpoints:
  GET  /health           liveness
  GET  /grib/datasets    list local files + their parameters/times
  POST /grib/extract     numeric values at points or over a bbox
  POST /grib/render      colormapped PNG tile of a field over a bbox
  POST /nowcast/run      extrapolate radar frames into nowcast frames
"""

from typing import Optional

from fastapi import FastAPI, HTTPException
from fastapi.responses import Response
from pydantic import BaseModel, model_validator

from . import geotiff, grib, nowcast, render, sources

app = FastAPI(title="gribsvc", version="0.1.0")


def _backend(path):
    """Pick the parser module by file type: radar GeoTIFFs vs GRIB."""
    return geotiff if geotiff.is_geotiff(path) else grib


class Point(BaseModel):
    lat: float
    lon: float


class BBox(BaseModel):
    min_lon: float
    min_lat: float
    max_lon: float
    max_lat: float

    def as_tuple(self) -> tuple[float, float, float, float]:
        return (self.min_lon, self.min_lat, self.max_lon, self.max_lat)


class ExtractRequest(BaseModel):
    file: str
    param: str
    time: Optional[str] = None
    points: Optional[list[Point]] = None
    bbox: Optional[BBox] = None
    step: int = 1

    @model_validator(mode="after")
    def _exactly_one_target(self):
        if bool(self.points) == bool(self.bbox):
            raise ValueError("provide exactly one of 'points' or 'bbox'")
        return self


class RenderRequest(BaseModel):
    file: str
    param: str
    time: Optional[str] = None
    bbox: BBox
    width: int = 256
    height: int = 256
    colormap: str = "jet"
    vmin: Optional[float] = None
    vmax: Optional[float] = None


@app.get("/health")
def health():
    return {"status": "ok"}


@app.get("/grib/datasets")
def datasets():
    out = []
    for name in sources.list_files():
        path = sources.resolve(name)
        try:
            params = _backend(path).list_params(path)
        except grib.GribError as exc:
            out.append({"file": name, "error": str(exc)})
            continue
        out.append({"file": name, "messages": params})
    return {"data_dir": str(sources.config.grib_data_dir), "datasets": out}


@app.post("/grib/extract")
def extract(req: ExtractRequest):
    try:
        path = sources.resolve(req.file)
        at = grib.parse_time(req.time)
        backend = _backend(path)
        if req.points:
            pts = [(p.lat, p.lon) for p in req.points]
            return backend.extract_points(path, req.param, pts, at)
        return backend.extract_bbox(path, req.param, req.bbox.as_tuple(), req.step, at)
    except sources.SourceError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    except grib.GribError as exc:
        raise HTTPException(status_code=422, detail=str(exc))
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))


class NowcastRequest(BaseModel):
    leads: int = nowcast.DEFAULT_LEADS


@app.post("/nowcast/run")
def nowcast_run(req: Optional[NowcastRequest] = None):
    leads = req.leads if req else nowcast.DEFAULT_LEADS
    try:
        return nowcast.run(leads=leads)
    except grib.GribError as exc:
        raise HTTPException(status_code=422, detail=str(exc))


@app.post("/grib/render")
def render_tile(req: RenderRequest):
    try:
        path = sources.resolve(req.file)
        at = grib.parse_time(req.time)
        values, lats, lons, meta = _backend(path).field_grid(path, req.param, at)
        png = render.render_png(
            values, lats, lons, req.bbox.as_tuple(),
            req.width, req.height, req.colormap, req.vmin, req.vmax,
        )
    except sources.SourceError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    except grib.GribError as exc:
        raise HTTPException(status_code=422, detail=str(exc))
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))

    headers = {"Cache-Control": "public, max-age=120"}
    if meta.get("valid_time"):
        headers["X-Data-Time"] = meta["valid_time"]
    return Response(content=png, media_type="image/png", headers=headers)
