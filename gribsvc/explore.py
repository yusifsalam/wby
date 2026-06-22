"""Interactive GRIB exploration scratchpad.

Open this file in PyCharm and run it cell-by-cell: each `# %%` marks a cell —
click the "Run Cell" gutter icon or press Ctrl+Enter. Variables persist in the
Python Console between cells, so you can poke at them as you go.

Requires the project interpreter set to gribsvc/.venv (numpy, pillow, pygrib).
"""

# %% setup — make `app` importable and point at the data dir regardless of cwd
import os
from pathlib import Path

HERE = Path(__file__).resolve().parent
os.environ["GRIB_DATA_DIR"] = str(HERE / "testdata")

import numpy as np
import pygrib

from app import grib, render, sources

# %% what files / fields are available
files = sources.list_files()
print("files:", files)

path = sources.resolve(files[0])
params = grib.list_params(path)
print("messages:", len(params))
for p in params[:3]:
    print(p)

# %% open the file directly with pygrib and inspect one message
grbs = pygrib.open(str(path))
msg = grbs[1]  # 1-indexed
print(msg)                       # one-line summary
print("shortName:", msg.shortName, "| name:", msg.name, "| units:", msg.units)
print("validDate:", msg.validDate, "| level:", msg.level, msg.typeOfLevel)

values = msg.values              # 2D numpy (masked) array
lats, lons = msg.latlons()
print("grid shape:", values.shape)
print("lat range:", float(lats.min()), "->", float(lats.max()))
print("lon range:", float(lons.min()), "->", float(lons.max()))
print("value range:", float(values.min()), "->", float(values.max()))

# %% list every GRIB key on a message (great for discovering metadata)
for key in sorted(msg.keys()):
    try:
        print(f"{key} = {msg[key]}")
    except Exception:
        pass
grbs.close()

# %% point extraction via the service helper (Kelvin -> Celsius for 2t)
# NOTE: this file's grid only covers lat 59.7-61.5, lon 19.1-29.8 (~2.5 km).
# Points outside that box snap to the nearest edge gridpoint.
pts = grib.extract_points(path, "2t", [(60.17, 24.94), (61.50, 23.76)], None)
for pt in pts["points"]:
    c = None if pt["value"] is None else round(pt["value"] - 273.15, 2)
    print(f"({pt['lat']},{pt['lon']}) -> {c} °C  @grid ({pt['grid_lat']:.2f},{pt['grid_lon']:.2f})")

# %% a specific forecast hour (time filter), as a numpy array you can plot
at = grib.parse_time("2026-05-17T00:00:00Z")
vals, vlats, vlons, meta = grib.field_grid(path, "2t", at)
print("meta:", meta, "| shape:", vals.shape)
celsius = vals - 273.15
print("min/mean/max °C:", round(float(celsius.min()), 1),
      round(float(celsius.mean()), 1), round(float(celsius.max()), 1))

# %% render a tile over the grid's coverage and view it
# bbox matches this file's extent; widen only within lat 59.7-61.5, lon 19.1-29.8.
png = render.render_png(
    vals, vlats, vlons,
    bbox=(19.1, 59.7, 29.8, 61.5),
    width=512, height=512, colormap="jet",
)
out_png = HERE / "tile.png"
out_png.write_bytes(png)
print("wrote", out_png)

from PIL import Image           # noqa: E402
Image.open(out_png).show()      # opens in your default image viewer

# %% OPTIONAL: nicer inline plot if you `pip install matplotlib`
# import matplotlib.pyplot as plt
# plt.imshow(celsius, origin="upper", cmap="jet")
# plt.colorbar(label="°C"); plt.title(meta["valid_time"]); plt.show()
