import type { MeasureKind } from "./formatters";
import type { HourlyForecast } from "./weatherApi";

export const HOURLY_METRIC_KEYS = [
  "feels",
  "wind",
  "gust",
  "direction",
  "precip",
  "pop",
  "humidity",
  "pressure",
  "cloud",
] as const;
export type HourlyMetricKey = (typeof HOURLY_METRIC_KEYS)[number];

export type HourlyMetric = {
  key: HourlyMetricKey;
  label: string;
  measure: MeasureKind;
  value: (hour: HourlyForecast) => number | null | undefined;
};

// Display order of the rows under each hour. Adding a metric here also needs a
// matching visibility rule in global.css (.hourly-metric[data-metric=...]).
export const HOURLY_METRICS: readonly HourlyMetric[] = [
  {
    key: "feels",
    label: "Feels like",
    measure: "temperature",
    value: (h) => h.feels_like,
  },
  { key: "wind", label: "Wind", measure: "speed", value: (h) => h.wind_speed },
  {
    key: "gust",
    label: "Wind gusts",
    measure: "speed",
    value: (h) => h.wind_gust,
  },
  {
    key: "direction",
    label: "Wind direction",
    measure: "direction",
    value: (h) => h.wind_direction,
  },
  {
    key: "precip",
    label: "Precipitation",
    measure: "precipitation",
    value: (h) => h.precipitation_1h,
  },
  {
    key: "pop",
    label: "Chance of rain",
    measure: "percent",
    value: (h) => h.pop,
  },
  {
    key: "humidity",
    label: "Humidity",
    measure: "percent",
    value: (h) => h.humidity,
  },
  {
    key: "pressure",
    label: "Pressure",
    measure: "pressure",
    value: (h) => h.pressure,
  },
  {
    key: "cloud",
    label: "Cloud cover",
    measure: "percent",
    value: (h) => h.cloud_cover,
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
