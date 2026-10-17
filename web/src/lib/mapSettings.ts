import type { PrecipRamp } from "./radarFrames";

// Map view settings persisted across visits, set on the full-page map
// (pages/map.astro) and followed by the precipitation widget
// (components/PrecipitationMap.astro).

const RAMP_STORAGE_KEY = "map-ramp";
const OPACITY_STORAGE_KEY = "map-opacity";
export const DEFAULT_OPACITY = 0.8;

function readStored(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeStored(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    // Storage unavailable (private mode, quota); the page still updates.
  }
}

export function parseRamp(raw: string | null): PrecipRamp {
  return raw === "stepped" ? "stepped" : "smooth";
}

export const storedRamp = (): PrecipRamp =>
  parseRamp(readStored(RAMP_STORAGE_KEY));

export function storeRamp(ramp: PrecipRamp) {
  writeStored(RAMP_STORAGE_KEY, ramp === "smooth" ? null : ramp);
}

export function storedOpacity(): number {
  const raw = readStored(OPACITY_STORAGE_KEY);
  const stored = raw === null ? NaN : Number(raw);
  return Number.isFinite(stored) && stored >= 0 && stored <= 1
    ? stored
    : DEFAULT_OPACITY;
}

export function storeOpacity(opacity: number) {
  writeStored(
    OPACITY_STORAGE_KEY,
    opacity === DEFAULT_OPACITY ? null : String(opacity),
  );
}
