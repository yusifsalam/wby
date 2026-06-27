export function formatTemperature(value: number | null | undefined): string {
  if (value == null) {
    return "--";
  }
  return `${Math.round(value)}°`;
}

export function formatSpeed(value: number | null | undefined): string {
  if (value == null) {
    return "--";
  }
  return `${Math.round(value)} m/s`;
}

export function formatPercent(value: number | null | undefined): string {
  if (value == null) {
    return "--";
  }
  return `${Math.round(value)}%`;
}

export function formatMillimeters(value: number | null | undefined): string {
  if (value == null) {
    return "--";
  }
  if (Math.abs(Math.round(value) - value) < 0.05) {
    return `${Math.round(value)} mm`;
  }
  return `${value.toFixed(1)} mm`;
}

export function formatVisibility(value: number | null | undefined): string {
  if (value == null) {
    return "--";
  }
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1)} km`;
  }
  return `${Math.round(value)} m`;
}

export function formatObservedTime(value: string, timeZone: string): string {
  return timeFormatter(timeZone).format(new Date(value));
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

function timeFormatter(timeZone: string): Intl.DateTimeFormat {
  return new Intl.DateTimeFormat("en-GB", {
    timeZone,
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}
