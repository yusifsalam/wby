"""Runtime configuration, read from environment variables.

Mirrors the Go server's simple env-var-with-fallback pattern
(internal/config/config.go) rather than pulling in a settings framework.
``grib_data_dir`` is read dynamically so it can be pointed at a fixture dir in
tests without re-importing.
"""

import os
from pathlib import Path


class Config:
    @property
    def grib_data_dir(self) -> Path:
        # Directory that GRIB2 files are resolved against. Phase 1 serves only
        # local files; phase 2 will also fetch from FMI into this dir.
        return Path(os.getenv("GRIB_DATA_DIR", "testdata")).resolve()

    @property
    def port(self) -> int:
        return int(os.getenv("PORT", "9090"))


config = Config()
