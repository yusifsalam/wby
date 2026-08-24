// Map layer identifiers, shared by browser map code (map.astro, mapOverlay)
// and the server-side proxy routes. Kept out of weatherApi.ts on purpose: that
// module imports node:crypto for request signing, so a runtime import of it
// from client code breaks the page in `astro dev` (no tree-shaking there).
export const MAP_LAYERS = [
  "temperature",
  "precipitation",
  "precipitation12h",
] as const;
export type MapLayer = (typeof MAP_LAYERS)[number];

export function isMapLayer(value: string): value is MapLayer {
  return (MAP_LAYERS as readonly string[]).includes(value);
}

// Layers backed by a server-rendered PNG at /v1/map/{layer}. precipitation12h
// is JSON-grid only (/v1/map/precipitation/forecast), rendered client-side.
export const OVERLAY_PNG_LAYERS = ["temperature", "precipitation"] as const;
export type OverlayPngLayer = (typeof OVERLAY_PNG_LAYERS)[number];

export function isOverlayPngLayer(value: string): value is OverlayPngLayer {
  return (OVERLAY_PNG_LAYERS as readonly string[]).includes(value);
}
