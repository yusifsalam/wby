import type { UVPoint } from "./weatherApi";

// WHO UV index bands, coloured like the iOS card.
export const UV_SCALE_MAX = 11;

export type UVBand = {
  label: string;
  color: string;
  limit: number;
};

export const UV_BANDS: readonly UVBand[] = [
  { label: "Low", color: "#70d65c", limit: 3 },
  { label: "Moderate", color: "#e6e040", limit: 6 },
  { label: "High", color: "#f29445", limit: 8 },
  { label: "Very high", color: "#f25257", limit: 11 },
  { label: "Extreme", color: "#cc4de6", limit: Number.POSITIVE_INFINITY },
];

export function uvBand(value: number): UVBand {
  return (
    UV_BANDS.find((band) => value < band.limit) ?? UV_BANDS[UV_BANDS.length - 1]
  );
}

export function uvLevel(value: number): string {
  return uvBand(value).label;
}

export function uvColor(value: number): string {
  return uvBand(value).color;
}

export type UVPeak = { value: number; time: string };

// Highest UV index among the points and the first hour it is reached.
export function uvPeak(points: readonly UVPoint[]): UVPeak | null {
  let peak: UVPeak | null = null;
  for (const point of points) {
    if (!Number.isFinite(point.uv)) continue;
    if (peak == null || point.uv > peak.value) {
      peak = { value: point.uv, time: point.time };
    }
  }
  return peak;
}

export type UVDayBar = {
  hour: number;
  time: string | null;
  uv: number | null;
  past: boolean;
  current: boolean;
};

type LocalTime = { date: string; hour: number };

function localTime(value: Date, timeZone: string): LocalTime {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    hour12: false,
  }).formatToParts(value);
  const get = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((part) => part.type === type)?.value ?? "";
  return {
    date: `${get("year")}-${get("month")}-${get("day")}`,
    hour: Number(get("hour")) % 24,
  };
}

// Points falling on the given local calendar day, one per hour. `offsetDays`
// selects today (0) or tomorrow (1) relative to `now`.
export function uvDayPoints(
  points: readonly UVPoint[],
  timeZone: string,
  now: Date,
  offsetDays = 0,
): UVPoint[] {
  const target = localTime(
    new Date(now.getTime() + offsetDays * 86_400_000),
    timeZone,
  ).date;
  return points.filter(
    (point) => localTime(new Date(point.time), timeZone).date === target,
  );
}

// The 24 hourly slots of today in the local time zone, for the histogram.
// Hours the forecast does not cover have a null value.
export function uvDayBars(
  points: readonly UVPoint[],
  timeZone: string,
  now: Date,
): UVDayBar[] {
  const current = localTime(now, timeZone);
  const bars: UVDayBar[] = Array.from({ length: 24 }, (_, hour) => ({
    hour,
    time: null,
    uv: null,
    past: hour < current.hour,
    current: hour === current.hour,
  }));
  for (const point of uvDayPoints(points, timeZone, now)) {
    const { hour } = localTime(new Date(point.time), timeZone);
    if (Number.isFinite(point.uv)) {
      bars[hour].time = point.time;
      bars[hour].uv = point.uv;
    }
  }
  return bars;
}

// Bar height as a percentage of the scale.
export function uvBarHeight(value: number): number {
  return (Math.min(Math.max(value, 0), UV_SCALE_MAX) / UV_SCALE_MAX) * 100;
}
