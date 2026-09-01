import type { LayerSpecification, Map as MapLibreMap } from "maplibre-gl";
import { FINLAND_RINGS } from "./finland-outline";
import type { MapLayer, OverlayPngLayer } from "./mapLayers";

// Shared map plumbing for the full-page map (pages/map.astro) and the
// precipitation widget (components/PrecipitationMap.astro): basemap styles and
// trimming, the fixed overlay extent + URL builder, the Mercator bake, and the
// frame manifest. Everything here is map-state free; the pages own their map,
// scrubber, and layer state.

export const BASEMAP_STYLES = {
  light: "https://tiles.openfreemap.org/styles/positron",
  dark: "https://tiles.openfreemap.org/styles/dark",
};

export const isDarkTheme = () =>
  document.documentElement.classList.contains("dark");

export const basemapStyleFor = () =>
  isDarkTheme() ? BASEMAP_STYLES.dark : BASEMAP_STYLES.light;

// 1×1 transparent PNG an image source is seeded with until the first frame loads.
export const TRANSPARENT_PNG =
  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=";

export type Corners = [
  [number, number],
  [number, number],
  [number, number],
  [number, number],
];
// warp marks a frame rasterized linear-in-latitude (grid renders) that needs
// the Mercator warp at bake time; absent means the layer's default applies.
export type Frame = { url: string; corners: Corners; warp?: boolean };

// Frame manifest: the server publishes the discrete scrubbable instants per
// layer (index of "now", step). Snapping the scrubber to these instants keeps
// overlay URLs stable across scrubs so HTTP caches actually hit. Falls back to
// a client-derived schedule if the manifest can't be fetched.
export type FrameSet = {
  times: string[];
  nowIndex: number;
  stepSeconds: number;
};
export type Manifest = Record<MapLayer, FrameSet>;

export function fallbackFrameSet(
  layer: MapLayer,
  nowMs = Date.now(),
): FrameSet {
  if (layer === "temperature") {
    const base = Math.floor(nowMs / 3_600_000) * 3_600_000;
    // now + 48 forecast hours, matching the server's MapForecastHorizon.
    return {
      times: Array.from({ length: 49 }, (_, i) =>
        new Date(base + i * 3_600_000).toISOString(),
      ),
      nowIndex: 0,
      stepSeconds: 3600,
    };
  }
  if (layer === "precipitation12h") {
    const base = Math.floor(nowMs / 3_600_000) * 3_600_000;
    // now + 12 forecast hours at the Harmonie field's hourly cadence.
    return {
      times: Array.from({ length: 13 }, (_, i) =>
        new Date(base + i * 3_600_000).toISOString(),
      ),
      nowIndex: 0,
      stepSeconds: 3600,
    };
  }
  const base = Math.floor(nowMs / 300_000) * 300_000;
  return {
    times: Array.from({ length: 25 }, (_, i) =>
      new Date(base + (i - 12) * 300_000).toISOString(),
    ),
    nowIndex: 12,
    stepSeconds: 300,
  };
}

// Server frame-set shape. The backend's JSON contract is snake_case, so map it
// to the camelCase FrameSet the client uses — a bare `as FrameSet` cast leaves
// nowIndex/stepSeconds undefined at runtime (NaN offsets, a slider that can't
// seek).
export type FrameSetJSON = {
  times?: string[];
  now_index?: number;
  step_seconds?: number;
};

export function normalizeFrameSet(
  raw: FrameSetJSON | undefined,
  layer: MapLayer,
): FrameSet {
  if (
    !raw?.times?.length ||
    raw.now_index == null ||
    raw.step_seconds == null
  ) {
    return fallbackFrameSet(layer);
  }
  return {
    times: raw.times,
    nowIndex: raw.now_index,
    stepSeconds: raw.step_seconds,
  };
}

export function fallbackManifest(): Manifest {
  return {
    temperature: fallbackFrameSet("temperature"),
    precipitation: fallbackFrameSet("precipitation"),
    precipitation12h: fallbackFrameSet("precipitation12h"),
  };
}

// Fetch the frame manifest via the signed proxy; any failure yields the
// client-derived fallback so callers always get a usable schedule.
export async function loadManifest(): Promise<Manifest> {
  try {
    const res = await fetch("/api/map/frames");
    if (!res.ok) return fallbackManifest();
    const data = (await res.json()) as {
      temperature?: FrameSetJSON;
      precipitation?: FrameSetJSON;
      precipitation12h?: FrameSetJSON;
    };
    return {
      temperature: normalizeFrameSet(data.temperature, "temperature"),
      precipitation: normalizeFrameSet(data.precipitation, "precipitation"),
      precipitation12h: normalizeFrameSet(
        data.precipitation12h,
        "precipitation12h",
      ),
    };
  } catch (err) {
    console.error("frame manifest fetch failed", err);
    return fallbackManifest();
  }
}

// The `time` query value for a frame index, or null for the live "now" frame
// (rendered by omitting the param so the server applies its latest-observation
// + fallback logic). Snapped to the manifest, so identical instants share a
// cacheable overlay URL.
export function frameTimeParam(frames: FrameSet, index: number): string | null {
  if (index === frames.nowIndex) return null;
  return frames.times[index] ?? null;
}

// Scrubber label helpers. Offset from the live frame ("Now", "+30m", "−1h").
export function formatFrameOffset(frames: FrameSet, index: number): string {
  const steps = index - frames.nowIndex;
  if (steps === 0) return "Now";
  const min = (steps * frames.stepSeconds) / 60;
  const sign = min > 0 ? "+" : "−";
  const abs = Math.abs(min);
  return abs % 60 === 0 ? `${sign}${abs / 60}h` : `${sign}${abs}m`;
}

// Local clock time of a frame — HH:mm, prefixed with a short weekday when it
// falls on a different day than now (temperature reaches +48h).
export function formatFrameClock(
  frames: FrameSet,
  index: number,
  now = new Date(),
): string {
  const iso = frames.times[index];
  if (!iso) return "";
  const d = new Date(iso);
  const tz = "Europe/Helsinki";
  const time = d.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
    timeZone: tz,
  });
  // Compare calendar days in Helsinki time so the weekday prefix flips at
  // Finnish midnight regardless of the viewer's own timezone.
  const dayKey = (t: Date) => t.toLocaleDateString("en-CA", { timeZone: tz });
  return dayKey(d) === dayKey(now)
    ? time
    : `${d.toLocaleDateString([], { weekday: "short", timeZone: tz })} ${time}`;
}

// Fixed full-Finland overlay extent (lng/lat), covering FMI's radar/forecast
// coverage. Hardcoding it — instead of tracking the viewport — makes every
// overlay URL depend only on (layer, time): identical across pans, users, and
// sessions, so browser and server caches actually hit and the whole frame
// window is prefetchable (sataako-fi's fixed-image model). The server renders
// Mercator-correct — temperature rows interpolate in Mercator-Y and the WMS
// request is EPSG:3857 — so these lng/lat corners align on the basemap, and
// data outside Finland masks to transparent. Panning/zoom needs no refetch;
// MapLibre reprojects the fixed raster on the GPU.
export const FINLAND = { w: 10.2155, s: 56.7513, e: 37.3717, n: 71.2417 };
export const FINLAND_CORNERS: Corners = [
  [FINLAND.w, FINLAND.n],
  [FINLAND.e, FINLAND.n],
  [FINLAND.e, FINLAND.s],
  [FINLAND.w, FINLAND.s],
];

export const mercatorY = (lat: number) =>
  Math.log(Math.tan(Math.PI / 4 + (lat * Math.PI) / 360));
export const inverseMercatorLat = (y: number) =>
  (Math.atan(Math.sinh(y)) * 180) / Math.PI;

// Fixed render size, aspect-correct in Web Mercator and capped at the server's
// max overlay dimension (1600px), so the overlay isn't stretched.
export const FINLAND_H = 1600;
export const FINLAND_W = Math.round(
  (FINLAND_H * ((FINLAND.e - FINLAND.w) * (Math.PI / 180))) /
    (mercatorY(FINLAND.n) - mercatorY(FINLAND.s)),
);

// Server-rendered overlay PNG for `layer` at `time` (null = live frame) over
// the fixed Finland extent.
export function fixedFrame(layer: OverlayPngLayer, time: string | null): Frame {
  const q = new URLSearchParams({
    bbox: `${FINLAND.w},${FINLAND.s},${FINLAND.e},${FINLAND.n}`,
    width: String(FINLAND_W),
    height: String(FINLAND_H),
  });
  if (time) q.set("time", time);
  return { url: `/api/map/${layer}?${q.toString()}`, corners: FINLAND_CORNERS };
}

export function loadImage(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = reject;
    img.src = url;
  });
}

// Redraw an overlay image onto a Mercator-correct canvas spanning `corners`,
// returning a data URL. lng→x is linear, lat→y is Mercator — matching how
// MapLibre lays the image across the coordinate quad.
//
// `warp`: the source raster's rows are LINEAR in latitude (a client-rasterized
// GRIB grid), so resample them row-band by row-band into Mercator space. A
// plain stretch would misregister the field by ~0.2–0.7° of latitude (tens of
// km south at these latitudes), dragging cool offshore cells over coastal
// cities; the warp lands isotherms on the right pixels, matching the iOS Metal
// shader (inverseMercatorLat per fragment). The radar precipitation PNG skips
// it: the server already renders in Mercator, so it's copied whole.
//
// `clip`: apply the Finland outline (FINLAND_RINGS) as a clip path so the edge
// lines up with the basemap coastline. Temperature only — it's a land field;
// rain is real over the sea, so both precipitation layers bake unclipped.
//
// Every layer must flow through this bake even when copied verbatim: MapLibre's
// ImageSource.updateImage repaints on its own only for an already-decoded data
// URL. Handing it a remote URL (the raw /api/map/precipitation PNG) sets the
// texture but doesn't trigger a render until something else forces one — so a
// plain layer switch wouldn't show until you scrubbed or hit play.
export type BakeOptions = { warp?: boolean; clip?: boolean };

export function bakeOverlayImage(
  source: HTMLImageElement | HTMLCanvasElement,
  corners: Corners,
  { warp = false, clip = false }: BakeOptions = {},
): string {
  const [west, north] = corners[0];
  const [east, south] = corners[2];
  const myN = mercatorY(north);
  const myS = mercatorY(south);
  const h = FINLAND_H;
  const w = Math.max(
    1,
    Math.round((h * ((east - west) * (Math.PI / 180))) / (myN - myS)),
  );
  const canvas = document.createElement("canvas");
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("no 2d canvas context");
  if (clip) {
    const toX = (lng: number) => ((lng - west) / (east - west)) * w;
    const toY = (lat: number) => ((myN - mercatorY(lat)) / (myN - myS)) * h;
    ctx.beginPath();
    for (const ring of FINLAND_RINGS) {
      ring.forEach(([lng, lat], i) => {
        const x = toX(lng);
        const y = toY(lat);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      });
      ctx.closePath();
    }
    ctx.clip();
  }
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = "high";
  if (warp) {
    // Linear-lat source → Mercator dest, in two seamless passes. Warping the
    // small source directly in strips leaves dark seams: fractional-y strip
    // edges anti-alias against the transparent canvas. Instead:
    //   1. upscale the source once to a full-size linear-lat buffer (smooth,
    //      no internal seams), then
    //   2. remap it row by row — each integer dest row samples the source
    //      latitude for its Mercator y (inverseMercatorLat). Integer, fully
    //      contiguous 1px rows can't leave anti-aliased gaps.
    // The warped buffer is then composited once through the coastline clip.
    const lin = document.createElement("canvas");
    lin.width = w;
    lin.height = h;
    const lctx = lin.getContext("2d");
    const warp = document.createElement("canvas");
    warp.width = w;
    warp.height = h;
    const wctx = warp.getContext("2d");
    if (!lctx || !wctx) throw new Error("no 2d canvas context");
    lctx.imageSmoothingEnabled = true;
    lctx.imageSmoothingQuality = "high";
    lctx.drawImage(source, 0, 0, w, h);
    wctx.imageSmoothingEnabled = true;
    wctx.imageSmoothingQuality = "high";
    for (let y = 0; y < h; y++) {
      const lat = inverseMercatorLat(myN - ((y + 0.5) / h) * (myN - myS));
      const srcY = ((north - lat) / (north - south)) * h;
      wctx.drawImage(lin, 0, srcY, w, 1, 0, y, w, 1);
    }
    ctx.drawImage(warp, 0, 0);
  } else {
    ctx.drawImage(source, 0, 0, w, h);
  }
  return canvas.toDataURL("image/png");
}

// Where to slot the overlay in the basemap's layer stack, per weather layer.
//
// Temperature sits BELOW the (opaque) water fill: the sea and lakes then paint
// over it, hiding — crucially in dark mode, where water is near-black — the dark
// lake blobs a translucent overlay would otherwise reveal. Temperature is a land
// field, so hiding it over water reads correctly. Precipitation sits ABOVE the
// fills (below roads/borders/labels) so rain stays visible over the sea. Either
// way roads, municipality borders, and place names render on top and stay
// legible.
export function overlayBeforeId(
  map: MapLibreMap,
  forLayer: MapLayer,
): string | undefined {
  const layers = map.getStyle().layers ?? [];
  if (forLayer === "temperature") {
    const water = layers.find(
      (l) =>
        l.type === "fill" &&
        ("source-layer" in l ? l["source-layer"] : undefined) === "water",
    );
    if (water) return water.id;
  }
  for (const l of layers) {
    const src = "source-layer" in l ? l["source-layer"] : undefined;
    if (l.type === "symbol" || src === "transportation" || src === "boundary") {
      return l.id;
    }
  }
  return undefined;
}

// Basemap layer visibility, extracted from web/layers.json but keyed on the
// OpenMapTiles **source-layer** rather than layer id — source-layers are part of
// the tile schema and identical across our light (positron) and dark styles,
// whereas the styles name their layers differently (label_* vs place_*,
// boundary_2 vs boundary_country_*), so an id list only ever matches one theme.
//
// Visible: water fills, borders, place labels, and motorway/major roads.
// Hidden (any other source-layer): rivers, water names, land use, landcover,
// buildings, aeroways, minor/service/path roads, piers, railways, road labels,
// parks, POIs — so the weather overlay stays readable.
//
// The transportation source-layer mixes motorways, minor roads and railways, so
// it's the one case source-layer can't decide alone: we read the layer's own
// class filter (stable OpenMapTiles class names, not style-specific ids) and
// keep only motorways + major roads.
const KEEP_ROAD_CLASSES = [
  "motorway",
  "trunk",
  "primary",
  "secondary",
  "tertiary",
];

export function keepBasemapLayer(l: LayerSpecification): boolean {
  if (l.type === "background") return true;
  const src = "source-layer" in l ? l["source-layer"] : undefined;
  switch (src) {
    case "water": // sea + lakes (fills)
    case "boundary": // state + national borders
    case "place": // all place labels
      return true;
    case "transportation": {
      if (l.type !== "line") return false; // skip oneway arrows, pier fills
      const filter = JSON.stringify(
        ("filter" in l ? l.filter : undefined) ?? "",
      );
      return KEEP_ROAD_CLASSES.some((c) => filter.includes(`"${c}"`));
    }
    default:
      return false;
  }
}

// Re-apply our basemap tweaks after each (re)load: hide every layer outside the
// minimal inventory above, and in dark mode make the surviving place labels
// legible over the bright overlay — the stock dark style uses mid-grey text with
// a soft translucent halo that muddies against the temperature/precip colours,
// so brighten the text and give it a crisp opaque halo. Light mode reads fine
// and is left untouched. setStyle resets everything, so this must run on every
// style.load.
export function tuneBasemap(map: MapLibreMap) {
  const dark = isDarkTheme();
  for (const l of map.getStyle().layers ?? []) {
    const src = "source-layer" in l ? l["source-layer"] : undefined;
    if (!keepBasemapLayer(l)) {
      map.setLayoutProperty(l.id, "visibility", "none");
      continue;
    }
    if (dark && l.type === "symbol" && src === "place") {
      map.setPaintProperty(l.id, "text-color", "rgb(236,236,236)");
      map.setPaintProperty(l.id, "text-halo-color", "rgb(0,0,0)");
      map.setPaintProperty(l.id, "text-halo-width", 1.8);
      map.setPaintProperty(l.id, "text-halo-blur", 0);
    }
  }
}

// Re-run `onThemeChange` whenever the app-wide light/dark toggle flips the
// `dark` class on <html>. Returns the observer so callers can disconnect it.
export function observeTheme(onThemeChange: () => void): MutationObserver {
  const observer = new MutationObserver((records) => {
    for (const r of records) {
      if (r.attributeName === "class") {
        onThemeChange();
        return;
      }
    }
  });
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["class"],
  });
  return observer;
}
