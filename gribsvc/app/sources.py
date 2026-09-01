"""Resolving a requested ``file`` name to a GRIB2 path on disk.

Phase 1: local files only, under ``GRIB_DATA_DIR``. Phase 2 will add a
``fetch(...)`` that downloads GRIB from FMI open data into the same dir and
returns the path — keeping this module the single seam for file provenance so
the HTTP request contract does not change.
"""

from pathlib import Path

from .config import config

GRIB_SUFFIXES = (".grib2", ".grib", ".grb", ".grb2")
TIFF_SUFFIXES = (".tif", ".tiff")


class SourceError(Exception):
    """Raised when a requested file cannot be resolved to a readable GRIB path."""


def resolve(file: str) -> Path:
    """Resolve ``file`` against the data dir, guarding against path traversal."""
    if not file or file.strip() == "":
        raise SourceError("file is required")

    base = config.grib_data_dir
    candidate = (base / file).resolve()

    # Reject anything that escapes the data dir (e.g. "../../etc/passwd").
    if candidate != base and base not in candidate.parents:
        raise SourceError("file is outside the data directory")
    if not candidate.is_file():
        raise SourceError(f"file not found: {file}")
    return candidate


def list_files() -> list[str]:
    """List GRIB and radar GeoTIFF files available in the data dir (names only)."""
    base = config.grib_data_dir
    if not base.is_dir():
        return []
    suffixes = GRIB_SUFFIXES + TIFF_SUFFIXES
    return sorted(
        p.name for p in base.iterdir() if p.is_file() and p.suffix.lower() in suffixes
    )


# TODO(phase2): def fetch(producer: str, param: str, at: datetime) -> Path:
#     Download the matching GRIB from FMI open data into config.grib_data_dir
#     (cache by producer/param/time) and return the local path.
