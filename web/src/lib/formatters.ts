import {
  celsiusToFahrenheit,
  hectopascalsToInchesHg,
  METERS_PER_MILE,
  metersPerSecondToMph,
  metersToFeet,
  metersToMiles,
  millimetersToInches,
  type UnitSystem,
} from "./units";

export function formatTemperature(
  value: number | null | undefined,
  system: UnitSystem = "metric",
): string {
  if (value == null) {
    return "--";
  }
  const shown = system === "imperial" ? celsiusToFahrenheit(value) : value;
  return `${Math.round(shown)}°`;
}

export function formatSpeed(
  value: number | null | undefined,
  system: UnitSystem = "metric",
): string {
  if (value == null) {
    return "--";
  }
  if (system === "imperial") {
    return `${Math.round(metersPerSecondToMph(value))} mph`;
  }
  return `${Math.round(value)} m/s`;
}

export function formatPercent(value: number | null | undefined): string {
  if (value == null) {
    return "--";
  }
  return `${Math.round(value)}%`;
}

export function formatIndex(value: number | null | undefined): string {
  if (value == null) {
    return "--";
  }
  return `${Math.round(value)}`;
}

export function formatPrecipitation(
  value: number | null | undefined,
  system: UnitSystem = "metric",
): string {
  if (value == null) {
    return "--";
  }
  if (system === "imperial") {
    const inches = millimetersToInches(value);
    if (inches > 0 && inches < 0.005) {
      return "<0.01 in";
    }
    if (Math.abs(Math.round(inches) - inches) < 0.005) {
      return `${Math.round(inches)} in`;
    }
    return `${inches.toFixed(2)} in`;
  }
  if (Math.abs(Math.round(value) - value) < 0.05) {
    return `${Math.round(value)} mm`;
  }
  return `${value.toFixed(1)} mm`;
}

export function formatPressure(
  value: number | null | undefined,
  system: UnitSystem = "metric",
): string {
  if (value == null) {
    return "--";
  }
  if (system === "imperial") {
    return `${hectopascalsToInchesHg(value).toFixed(2)} inHg`;
  }
  return `${Math.round(value)} hPa`;
}

const COMPASS_POINTS = ["N", "NE", "E", "SE", "S", "SW", "W", "NW"] as const;

// Meteorological direction (degrees the wind blows *from*) as a compass point.
export function formatWindDirection(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) {
    return "--";
  }
  const index = Math.round((((value % 360) + 360) % 360) / 45) % 8;
  return COMPASS_POINTS[index];
}

export function formatVisibility(
  value: number | null | undefined,
  system: UnitSystem = "metric",
): string {
  if (value == null) {
    return "--";
  }
  if (system === "imperial") {
    if (value >= METERS_PER_MILE) {
      return `${metersToMiles(value).toFixed(1)} mi`;
    }
    return `${Math.round(metersToFeet(value))} ft`;
  }
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1)} km`;
  }
  return `${Math.round(value)} m`;
}

export const MEASURE_KINDS = [
  "temperature",
  "speed",
  "precipitation",
  "pressure",
  "visibility",
  "percent",
  "direction",
  "index",
] as const;
export type MeasureKind = (typeof MEASURE_KINDS)[number];

// One entry point for every unit-bearing value so the Measure component can
// render the same number in both systems.
export function formatMeasure(
  kind: MeasureKind,
  value: number | null | undefined,
  system: UnitSystem = "metric",
): string {
  switch (kind) {
    case "temperature":
      return formatTemperature(value, system);
    case "speed":
      return formatSpeed(value, system);
    case "precipitation":
      return formatPrecipitation(value, system);
    case "pressure":
      return formatPressure(value, system);
    case "visibility":
      return formatVisibility(value, system);
    case "percent":
      return formatPercent(value);
    case "direction":
      return formatWindDirection(value);
    case "index":
      return formatIndex(value);
  }
}

export function formatObservedTime(value: string, timeZone: string): string {
  return timeFormatter(timeZone).format(new Date(value));
}

export function formatRelativeTime(
  value: string,
  now: number = Date.now(),
): string {
  const seconds = Math.max(
    0,
    Math.round((now - new Date(value).getTime()) / 1000),
  );
  if (seconds < 60) {
    return "just now";
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes} min ago`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours}h ago`;
  }
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function hourLabel(value: string, timeZone: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    timeZone,
    hour: "2-digit",
    hour12: false,
  }).format(new Date(value));
}

export function dayLabel(value: string, timeZone: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    timeZone,
    weekday: "short",
  }).format(new Date(`${value}T12:00:00Z`));
}

export function localHour(value: string, timeZone: string): number {
  return Number.parseInt(
    new Intl.DateTimeFormat("en-GB", {
      timeZone,
      hour: "2-digit",
      hourCycle: "h23",
    }).format(new Date(value)),
    10,
  );
}

export function dayTitle(value: string, timeZone: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    timeZone,
    weekday: "long",
    day: "numeric",
    month: "long",
  }).format(new Date(`${value}T12:00:00Z`));
}

function timeFormatter(timeZone: string): Intl.DateTimeFormat {
  return new Intl.DateTimeFormat("en-GB", {
    timeZone,
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}
