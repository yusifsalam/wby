import os
from pathlib import Path

import pytest

TESTDATA = Path(__file__).resolve().parent.parent / "testdata"
GRIB_SUFFIXES = (".grib2", ".grib", ".grb", ".grb2")


def _first_grib() -> Path | None:
    if not TESTDATA.is_dir():
        return None
    files = sorted(p for p in TESTDATA.iterdir() if p.suffix.lower() in GRIB_SUFFIXES)
    return files[0] if files else None


@pytest.fixture
def sample_grib() -> Path:
    path = _first_grib()
    if path is None:
        pytest.skip("no .grib2 fixture in testdata/ (drop one in to enable these tests)")
    return path


@pytest.fixture
def first_param(sample_grib) -> str:
    """The shortName of the first message in the sample file."""
    from app import grib

    params = grib.list_params(sample_grib)
    if not params:
        pytest.skip("sample GRIB has no messages")
    return params[0]["param"]


@pytest.fixture(autouse=True)
def _point_data_dir():
    """Resolve files against testdata/ for the duration of a test."""
    os.environ["GRIB_DATA_DIR"] = str(TESTDATA)
    yield
