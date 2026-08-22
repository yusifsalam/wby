import {
  formatMillimeters,
  formatPercent,
  formatPressure,
  formatSpeed,
} from "./formatters";
import type { HourlyForecast } from "./weatherApi";

export const HOURLY_METRIC_KEYS = [
  "wind",
  "gust",
  "precip",
  "pressure",
  "cloud",
] as const;
export type HourlyMetricKey = (typeof HOURLY_METRIC_KEYS)[number];

export type HourlyMetric = {
  key: HourlyMetricKey;
  label: string;
  format: (hour: HourlyForecast) => string;
};

// Display order of the rows under each hour. Adding a metric here also needs a
// matching visibility rule in global.css (.hourly-metric[data-metric=...]).
export const HOURLY_METRICS: readonly HourlyMetric[] = [
  { key: "wind", label: "Wind", format: (h) => formatSpeed(h.wind_speed) },
  { key: "gust", label: "Wind gusts", format: (h) => formatSpeed(h.wind_gust) },
  {
    key: "precip",
    label: "Precipitation",
    format: (h) => formatMillimeters(h.precipitation_1h),
  },
  {
    key: "pressure",
    label: "Pressure",
    format: (h) => formatPressure(h.pressure),
  },
  {
    key: "cloud",
    label: "Cloud cover",
    format: (h) => formatPercent(h.cloud_cover),
  },
];

export const DEFAULT_HOURLY_METRICS: readonly HourlyMetricKey[] = [
  "wind",
  "precip",
];

export const HOURLY_METRICS_STORAGE_KEY = "hourly-metrics";

function isMetricKey(value: string): value is HourlyMetricKey {
  return (HOURLY_METRIC_KEYS as readonly string[]).includes(value);
}

// The selection is stored as a space-separated token list so the pre-paint
// inline script in BaseLayout can copy it verbatim onto
// <html data-hourly-metrics> and CSS can match tokens with
// [data-hourly-metrics~="key"]. `null` (nothing saved) means the defaults;
// an empty string is a deliberate "show nothing".
export function parseHourlyMetrics(
  raw: string | null | undefined,
): HourlyMetricKey[] {
  if (raw == null) {
    return [...DEFAULT_HOURLY_METRICS];
  }
  const tokens = new Set(raw.split(/\s+/).filter(isMetricKey));
  return HOURLY_METRIC_KEYS.filter((key) => tokens.has(key));
}

export function serializeHourlyMetrics(
  keys: readonly HourlyMetricKey[],
): string {
  const selected = new Set(keys);
  return HOURLY_METRIC_KEYS.filter((key) => selected.has(key)).join(" ");
}
