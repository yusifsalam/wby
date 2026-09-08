export const UNIT_SYSTEMS = ["metric", "imperial"] as const;
export type UnitSystem = (typeof UNIT_SYSTEMS)[number];

export const DEFAULT_UNIT_SYSTEM: UnitSystem = "metric";

// localStorage key; the value is mirrored onto <html data-units> pre-paint by
// BaseLayout and toggled by UnitsToggle. Only "imperial" is ever stored, so an
// absent key means metric.
export const UNITS_STORAGE_KEY = "units";

export function parseUnitSystem(raw: string | null | undefined): UnitSystem {
  return raw === "imperial" ? "imperial" : DEFAULT_UNIT_SYSTEM;
}

export function celsiusToFahrenheit(celsius: number): number {
  return (celsius * 9) / 5 + 32;
}

export function metersPerSecondToMph(value: number): number {
  return value * 2.236936;
}

export function millimetersToInches(value: number): number {
  return value / 25.4;
}

export function hectopascalsToInchesHg(value: number): number {
  return value * 0.02952998;
}

export const METERS_PER_MILE = 1609.344;

export function metersToMiles(value: number): number {
  return value / METERS_PER_MILE;
}

export function metersToFeet(value: number): number {
  return value / 0.3048;
}
