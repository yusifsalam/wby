import {
  type Corners,
  type Frame,
  type FrameSet,
  fixedFrame,
  frameTimeParam,
} from "./mapOverlay";

// Client-side grid rendering shared by the full-page map (pages/map.astro) and
// the precipitation widget (components/PrecipitationMap.astro): color ramps,
// grid rasterization, and the ±1h radar layer's frame source.

export type RGB = [number, number, number];
export type RGBA = [number, number, number, number];
export type RampStops = [number, RGB][];

export function rampColor(stops: RampStops, value: number): RGB {
  if (value <= stops[0][0]) return stops[0][1];
  const last = stops.length - 1;
  if (value >= stops[last][0]) return stops[last][1];
  for (let i = 0; i < last; i++) {
    const [at, a] = stops[i];
    const [bt, b] = stops[i + 1];
    if (value > bt) continue;
    const t = (value - at) / (bt - at);
    return [
      Math.round(a[0] + (b[0] - a[0]) * t),
      Math.round(a[1] + (b[1] - a[1]) * t),
      Math.round(a[2] + (b[2] - a[2]) * t),
    ];
  }
  return stops[last][1];
}

// 8-stop temperature ramp, matching the server's rampColorForTemperature
// (internal/weather/overlay.go) so the grid render agrees with the PNG/iOS.
export const TEMP_STOPS: RampStops = [
  [-40, [80, 30, 130]],
  [-20, [30, 55, 150]],
  [-10, [55, 115, 220]],
  [0, [80, 190, 180]],
  [10, [180, 215, 75]],
  [20, [245, 210, 55]],
  [30, [210, 50, 40]],
  [40, [109, 22, 11]],
];

export const temperatureRGBA = (v: number): RGBA => [
  ...rampColor(TEMP_STOPS, v),
  255,
];

// Stepped temperature: 2 °C bands on even degrees, each painted with the
// smooth ramp's color at the band midpoint. Matches the iOS Metal shader
// (TemperatureShader.metal, kTempBandWidth).
export const TEMP_BAND_WIDTH = 2;

export const temperatureSteppedRGBA = (v: number): RGBA =>
  temperatureRGBA((Math.floor(v / TEMP_BAND_WIDTH) + 0.5) * TEMP_BAND_WIDTH);

// Precipitation ramp + alpha for the grids, approximating the FMI WMS radar
// palette of the PNG frames (see the shared legend) so every precipitation
// layer reads as one scheme. The alpha treatment follows the iOS Metal shader
// (TemperatureShader.metal, precipitation_grid_fragment): below PRECIP_DRY a
// cell is transparent, and alpha ramps from 45% at 0.1 mm/h to full at
// 1 mm/h. Dry cells keep the first stop's RGB so the GPU's bilinear upscale
// blends toward light blue at rain edges instead of fringing dark.
const PRECIP_STOPS: RampStops = [
  [0.1, [168, 224, 255]],
  [1, [63, 160, 255]],
  [5, [20, 54, 255]],
  [20, [139, 0, 255]],
  [50, [255, 0, 170]],
];
const PRECIP_DRY = 0.1;
const PRECIP_OPAQUE = 1;
const PRECIP_MIN_ALPHA = 0.45;

export function precipitationRGBA(v: number): RGBA {
  const [r, g, b] = rampColor(PRECIP_STOPS, v);
  if (v < PRECIP_DRY) return [r, g, b, 0];
  const intensity =
    v >= PRECIP_OPAQUE ? 1 : (v - PRECIP_DRY) / (PRECIP_OPAQUE - PRECIP_DRY);
  return [
    r,
    g,
    b,
    Math.round(255 * (PRECIP_MIN_ALPHA + (1 - PRECIP_MIN_ALPHA) * intensity)),
  ];
}

// FMI's Mobile_dark radar classes: 8 discrete bands at a constant ~70%
// alpha, colors and thresholds lifted from the WMS legend so the grid render
// is indistinguishable from the official tiles.
const STEPPED_CLASS_EDGES = [0.1, 0.3, 0.5, 1, 2.5, 5, 10, 15];
const STEPPED_CLASS_COLORS: RGB[] = [
  [173, 243, 250],
  [134, 190, 243],
  [95, 138, 236],
  [57, 85, 229],
  [20, 33, 222],
  [130, 56, 236],
  [185, 67, 244],
  [241, 78, 251],
];
const STEPPED_CLASS_ALPHA = 179;

export function precipitationSteppedRGBA(v: number): RGBA {
  if (v < STEPPED_CLASS_EDGES[0]) return [0, 0, 0, 0];
  let cls = STEPPED_CLASS_EDGES.length - 1;
  for (let i = 1; i < STEPPED_CLASS_EDGES.length; i++) {
    if (v < STEPPED_CLASS_EDGES[i]) {
      cls = i - 1;
      break;
    }
  }
  const [r, g, b] = STEPPED_CLASS_COLORS[cls];
  return [r, g, b, STEPPED_CLASS_ALPHA];
}

export type MapRamp = "smooth" | "stepped";

export const precipitationRampRGBA = (ramp: MapRamp) =>
  ramp === "stepped" ? precipitationSteppedRGBA : precipitationRGBA;

export const temperatureRampRGBA = (ramp: MapRamp) =>
  ramp === "stepped" ? temperatureSteppedRGBA : temperatureRGBA;

export type FieldGrid = {
  rows: number;
  cols: number;
  min_lat: number;
  max_lat: number;
  min_lon: number;
  max_lon: number;
  values: (number | null)[];
};

// Rasterize the dense GRIB grid to a cols×rows canvas colored by the given
// colorizer (masked cells transparent). MapLibre's raster layer
// bilinear-upscales the small texture on the GPU, giving the smooth field iOS
// renders with Metal — instead of flattening the grid back into IDW points
// server-side.
export function rasterizeGrid(
  grid: FieldGrid,
  rgba: (v: number) => RGBA,
): string {
  const { rows, cols, values } = grid;
  const canvas = document.createElement("canvas");
  canvas.width = cols;
  canvas.height = rows;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("no 2d canvas context");
  const img = ctx.createImageData(cols, rows);
  // values are row-major north-to-south (row 0 = max_lat), so index maps
  // directly to canvas pixels with y=0 at the top (north).
  for (let i = 0; i < values.length; i++) {
    const v = values[i];
    const o = i * 4;
    if (v == null) {
      img.data[o + 3] = 0;
      continue;
    }
    const [r, g, b, a] = rgba(v);
    img.data[o] = r;
    img.data[o + 1] = g;
    img.data[o + 2] = b;
    img.data[o + 3] = a;
  }
  ctx.putImageData(img, 0, 0);
  return canvas.toDataURL("image/png");
}

export function gridCorners(grid: FieldGrid): Corners {
  return [
    [grid.min_lon, grid.max_lat],
    [grid.max_lon, grid.max_lat],
    [grid.max_lon, grid.min_lat],
    [grid.min_lon, grid.min_lat],
  ];
}

// Precipitation grids are fetched over the full fixed map canvas (FINLAND,
// slightly padded — the server downloads GRIB over this same extent,
// GRIB_BBOX) so they span the same area as the radar PNGs; the response's own
// corners are used, so a model-domain edge inside this bbox still renders
// correctly. Width/height are required by the endpoint but unused for the grid
// payload; keeping them constant keeps the server cache key stable.
export const PRECIP_GRID_QUERY = new URLSearchParams({
  bbox: "10,56.5,37.6,71.5",
  width: "600",
  height: "600",
}).toString();

// Nowcast frames are re-predicted every run (~5 min), so unlike observation
// frames they can't be memoized for the page lifetime; entries expire after
// the proxy's cache window and re-rasterize from the browser HTTP cache.
const NOWCAST_FRAME_TTL_MS = 60_000;

// Frame source for the ±1h precipitation layer: past/now frames render the
// radar observation grid, future frames the extrapolation nowcast grid, and a
// grid miss falls back to the server PNG. Rasterized frames bake the ramp in,
// so cache keys carry it.
export function createRadarFrames(ramp: () => MapRamp) {
  const observedCache = new Map<string, Frame | null>();
  const nowcastCache = new Map<
    string,
    { frame: Frame | null; fetchedAt: number }
  >();
  const pngPreloads = new Map<string, HTMLImageElement>();

  const isPast = (at: string) => Date.parse(at) <= Date.now();

  // 404 = frame not on the server (radar disabled, or a gap) — a known miss.
  async function fetchGridFrame(
    endpoint: "precipitation-observed" | "precipitation-nowcast",
    at: string,
    style: MapRamp,
  ): Promise<Frame | null> {
    const res = await fetch(
      `/api/map/${endpoint}?${PRECIP_GRID_QUERY}&time=${encodeURIComponent(at)}`,
    );
    if (res.status === 404) return null;
    if (!res.ok)
      throw new Error(`${endpoint} grid fetch failed: ${res.status}`);
    const data = (await res.json()) as { grid?: FieldGrid | null };
    if (!data.grid || data.grid.values.length === 0) return null;
    return {
      url: rasterizeGrid(data.grid, precipitationRampRGBA(style)),
      corners: gridCorners(data.grid),
      warp: true,
    };
  }

  function cached(at: string, style: MapRamp): Frame | null | undefined {
    const key = `${at}|${style}`;
    if (isPast(at)) return observedCache.get(key);
    const entry = nowcastCache.get(key);
    if (!entry) return undefined;
    if (Date.now() - entry.fetchedAt > NOWCAST_FRAME_TTL_MS) {
      nowcastCache.delete(key);
      return undefined;
    }
    return entry.frame;
  }

  async function gridFrame(at: string): Promise<Frame | null> {
    const style = ramp();
    const hit = cached(at, style);
    if (hit !== undefined) return hit;
    const key = `${at}|${style}`;
    if (isPast(at)) {
      const frame = await fetchGridFrame("precipitation-observed", at, style);
      observedCache.set(key, frame);
      return frame;
    }
    const frame = await fetchGridFrame("precipitation-nowcast", at, style);
    nowcastCache.set(key, { frame, fetchedAt: Date.now() });
    return frame;
  }

  const pngFrame = (frames: FrameSet, index: number): Frame =>
    fixedFrame("precipitation", frameTimeParam(frames, index));

  async function frameAt(frames: FrameSet, index: number): Promise<Frame> {
    const at = frames.times[index];
    if (at) {
      try {
        const grid = await gridFrame(at);
        if (grid) return grid;
      } catch (err) {
        console.error("radar grid fetch failed, using PNG", err);
      }
    }
    return pngFrame(frames, index);
  }

  // Fire-and-forget: warm the grid cache for a frame, preloading the fallback
  // PNG into the browser HTTP cache on a miss.
  function prefetch(frames: FrameSet, index: number) {
    const at = frames.times[index];
    if (!at || cached(at, ramp()) !== undefined) return;
    gridFrame(at)
      .then((frame) => {
        if (frame) return;
        const { url } = pngFrame(frames, index);
        if (pngPreloads.has(url)) return;
        const img = new Image();
        img.src = url;
        pngPreloads.set(url, img);
      })
      .catch(() => {});
  }

  return { frameAt, prefetch };
}
