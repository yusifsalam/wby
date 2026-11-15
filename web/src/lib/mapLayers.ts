// Map layer identifiers, shared by browser map code (map.astro, mapOverlay).
// Kept out of weatherApi.ts on purpose: that module imports node:crypto for
// request signing, so a runtime import of it from client code breaks the page
// in `astro dev` (no tree-shaking there).
export const MAP_LAYERS = [
  "temperature",
  "precipitation",
  "precipitation12h",
] as const;
export type MapLayer = (typeof MAP_LAYERS)[number];

export function isMapLayer(value: string): value is MapLayer {
  return (MAP_LAYERS as readonly string[]).includes(value);
}
