import { metersPerSecondToMph, type UnitSystem } from "./units";
import type {
  CurrentConditions,
  DailyClimateNormalsResponse,
  DailyForecast,
} from "./weatherApi";

export type LocalDate = {
  date: string;
  month: number;
  day: number;
  hour: number;
  offsetHours: number;
};

// Calendar date, hour and UTC offset of `now` in the given time zone.
export function localDate(now: Date, timeZone: string): LocalDate {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).formatToParts(now);
  const get = (type: Intl.DateTimeFormatPartTypes) =>
    Number(parts.find((part) => part.type === type)?.value ?? "0");
  const year = get("year");
  const month = get("month");
  const day = get("day");
  const hour = get("hour") % 24;
  const wall = Date.UTC(year, month - 1, day, hour, get("minute"));
  const seconds = Math.floor(now.getTime() / 1000) * 1000;
  return {
    date: `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`,
    month,
    day,
    hour,
    offsetHours: Math.round((wall - seconds) / 3_600_000),
  };
}

// A 24-value UTC-indexed hourly normal rotated to local hours.
export function localizeHourly(
  utc: readonly number[] | null | undefined,
  offsetHours: number,
): number[] {
  if (utc?.length !== 24) return [];
  return Array.from(
    { length: 24 },
    (_, hour) => utc[(((hour - offsetHours) % 24) + 24) % 24],
  );
}

export type NormalsSeries = {
  mode: "hourly" | "daily";
  avgs: number[];
  highs: number[];
  lows: number[];
  todayIndex: number;
  labels: { index: number; label: string }[];
};

// Today's hourly mean with its 10th–90th percentile band, in local hours.
export function normalsHourlySeries(
  normals: DailyClimateNormalsResponse,
  timeZone: string,
  now: Date,
): NormalsSeries | null {
  const today = normals.today;
  if (today.temp_hourly?.length !== 24) return null;
  const local = localDate(now, timeZone);
  return {
    mode: "hourly",
    avgs: localizeHourly(today.temp_hourly, local.offsetHours),
    highs: localizeHourly(today.temp_hourly_p90, local.offsetHours),
    lows: localizeHourly(today.temp_hourly_p10, local.offsetHours),
    todayIndex: local.hour,
    labels: [0, 6, 12, 18].map((hour) => ({
      index: hour,
      label: String(hour).padStart(2, "0"),
    })),
  };
}

// This month's daily mean with its normal high and low.
export function normalsDailySeries(
  normals: DailyClimateNormalsResponse,
  timeZone: string,
  now: Date,
): NormalsSeries | null {
  const local = localDate(now, timeZone);
  const days = normals.daily
    .filter((day) => day.month === local.month && day.temp_avg != null)
    .sort((a, b) => a.day - b.day);
  if (days.length === 0) return null;
  const hasRange = days.every(
    (day) => day.temp_high != null && day.temp_low != null,
  );
  let todayIndex = days.findIndex((day) => day.day >= local.day);
  if (todayIndex < 0) todayIndex = days.length - 1;
  const last = days[days.length - 1].day;
  const labelDays = [1, 8, 15, 22, last];
  return {
    mode: "daily",
    avgs: days.map((day) => day.temp_avg as number),
    highs: hasRange ? days.map((day) => day.temp_high as number) : [],
    lows: hasRange ? days.map((day) => day.temp_low as number) : [],
    todayIndex,
    labels: days.flatMap((day, index) =>
      labelDays.includes(day.day) ? [{ index, label: String(day.day) }] : [],
    ),
  };
}

// The card's curve: hourly when the station has one, else daily.
export function normalsSeries(
  normals: DailyClimateNormalsResponse,
  timeZone: string,
  now: Date,
): NormalsSeries | null {
  return (
    normalsHourlySeries(normals, timeZone, now) ??
    normalsDailySeries(normals, timeZone, now)
  );
}

export type ChartMarker = { x: number; y: number; side: "left" | "right" };

export type NormalsChart = {
  mode: NormalsSeries["mode"];
  min: number;
  max: number;
  ticks: { value: number; y: number }[];
  bandPath: string;
  linePath: string;
  today: {
    x: number;
    meanY: number;
    highY: number | null;
    lowY: number | null;
  };
  now: ChartMarker | null;
  forecastHigh: ChartMarker | null;
  forecastLow: ChartMarker | null;
  labels: { x: number; label: string; today: boolean }[];
};

export type ForecastMarkers = {
  high?: number | null;
  low?: number | null;
};

// Chart geometry in percentages: `x` from the left, `y` from the bottom. The
// SVG paths use the same numbers with y flipped for a 0 0 100 100 viewBox.
export function buildNormalsChart(
  series: NormalsSeries,
  currentTemp: number | null | undefined,
  forecast: ForecastMarkers = {},
): NormalsChart {
  const values = [...series.avgs, ...series.highs, ...series.lows];
  for (const extra of [currentTemp, forecast.high, forecast.low]) {
    if (extra != null) values.push(extra);
  }
  const min = Math.floor(Math.min(...values)) - 2;
  const max = Math.ceil(Math.max(...values)) + 2;
  const count = series.avgs.length;
  const x = (index: number) => ((index + 0.5) / count) * 100;
  const y = (value: number) => ((value - min) / (max - min)) * 100;

  const step = niceStep((max - min) / 5);
  const ticks: NormalsChart["ticks"] = [];
  for (let value = Math.ceil(min / step) * step; value <= max; value += step) {
    ticks.push({ value, y: y(value) });
  }

  const point = (index: number, value: number) =>
    `${fmt(x(index))},${fmt(100 - y(value))}`;
  const linePath = series.avgs
    .map((value, i) => `${i === 0 ? "M" : "L"}${point(i, value)}`)
    .join("");
  const hasRange =
    series.highs.length === count && series.lows.length === count;
  let bandPath = "";
  if (hasRange) {
    const top = series.highs.map(
      (value, i) => `${i === 0 ? "M" : "L"}${point(i, value)}`,
    );
    const bottom: string[] = [];
    for (let i = count - 1; i >= 0; i--) {
      bottom.push(`L${point(i, series.lows[i])}`);
    }
    bandPath = `${top.join("")}${bottom.join("")}Z`;
  }

  const todayX = x(series.todayIndex);
  const marker = (value: number | null | undefined): ChartMarker | null =>
    value == null
      ? null
      : { x: todayX, y: y(value), side: todayX > 65 ? "left" : "right" };
  return {
    mode: series.mode,
    min,
    max,
    ticks,
    bandPath,
    linePath,
    today: {
      x: todayX,
      meanY: y(series.avgs[series.todayIndex]),
      highY: hasRange ? y(series.highs[series.todayIndex]) : null,
      lowY: hasRange ? y(series.lows[series.todayIndex]) : null,
    },
    now: marker(currentTemp),
    forecastHigh: marker(forecast.high),
    forecastLow: marker(forecast.low),
    labels: series.labels.map(({ index, label }) => ({
      x: x(index),
      label,
      today: index === series.todayIndex,
    })),
  };
}

// 1, 2 or 5 times a power of ten, at or above `raw`.
export function niceStep(raw: number): number {
  if (!(raw > 0)) return 1;
  const magnitude = 10 ** Math.floor(Math.log10(raw));
  const fraction = raw / magnitude;
  const nice = fraction <= 1 ? 1 : fraction <= 2 ? 2 : fraction <= 5 ? 5 : 10;
  return nice * magnitude;
}

function fmt(value: number): string {
  return String(Math.round(value * 100) / 100);
}

export type StatKind =
  | "temperature"
  | "speed"
  | "percent"
  | "precipitation"
  | "snow";
export type DeltaTone = "warm" | "cold" | "even";

type UnitSpec = {
  scale: number;
  offset: number;
  unit: string;
  decimals?: number;
};

function unitSpec(kind: StatKind, system: UnitSystem): UnitSpec {
  const imperial = system === "imperial";
  switch (kind) {
    case "temperature":
      return imperial
        ? { scale: 9 / 5, offset: 32, unit: "°" }
        : { scale: 1, offset: 0, unit: "°" };
    case "speed":
      return imperial
        ? { scale: metersPerSecondToMph(1), offset: 0, unit: " mph" }
        : { scale: 1, offset: 0, unit: " m/s" };
    case "percent":
      return { scale: 1, offset: 0, unit: "%", decimals: 0 };
    case "precipitation":
      return imperial
        ? { scale: 1 / 25.4, offset: 0, unit: " in", decimals: 2 }
        : { scale: 1, offset: 0, unit: " mm" };
    case "snow":
      return imperial
        ? { scale: 1 / 2.54, offset: 0, unit: " in", decimals: 1 }
        : { scale: 1, offset: 0, unit: " cm" };
  }
}

export function roundToStep(value: number, step: number): number {
  return Math.round(value / step) * step;
}

export function formatNumber(value: number): string {
  return String(Number(value.toFixed(3)));
}

// A value with its unit in the given system, e.g. "17.7°", "4 m/s", "0.08 in".
export function formatStat(
  kind: StatKind,
  value: number,
  system: UnitSystem,
  decimals = 0,
): string {
  const spec = unitSpec(kind, system);
  const shown = value * spec.scale + spec.offset;
  return `${shown.toFixed(spec.decimals ?? decimals)}${spec.unit}`;
}

// The difference in the given system, rounded to the step shown at that
// precision: half degrees and whole units at 0 decimals, else tenths (or the
// unit's own precision).
export function deltaValue(
  kind: StatKind,
  diff: number,
  system: UnitSystem,
  decimals = 0,
): number {
  const spec = unitSpec(kind, system);
  const precision = spec.decimals ?? decimals;
  const step =
    precision > 0 ? 10 ** -precision : kind === "temperature" ? 0.5 : 1;
  return roundToStep(diff * spec.scale, step);
}

export function deltaTone(
  kind: StatKind,
  diff: number,
  decimals = 0,
): DeltaTone {
  const rounded = deltaValue(kind, diff, "metric", decimals);
  if (rounded > 0) return "warm";
  if (rounded < 0) return "cold";
  return "even";
}

// "+1.5°", "−2 m/s", "±0°" (U+2212 minus, like the iOS card).
export function formatDelta(
  kind: StatKind,
  diff: number,
  system: UnitSystem,
  decimals = 0,
): string {
  const value = deltaValue(kind, diff, system, decimals);
  const unit = unitSpec(kind, system).unit;
  if (value > 0) return `+${formatNumber(value)}${unit}`;
  if (value < 0) return `−${formatNumber(-value)}${unit}`;
  return `±0${unit}`;
}

export type NormalsHeadline = {
  current: number;
  normal: number;
  diff: number;
  tone: DeltaTone;
};

export function normalsHeadline(
  currentTemp: number | null | undefined,
  tempNowNormal: number | null | undefined,
  decimals = 0,
): NormalsHeadline | null {
  if (currentTemp == null || tempNowNormal == null) return null;
  const diff = currentTemp - tempNowNormal;
  return {
    current: currentTemp,
    normal: tempNowNormal,
    diff,
    tone: deltaTone("temperature", diff, decimals),
  };
}

export function headlineText(
  diff: number,
  system: UnitSystem,
  decimals = 0,
): string {
  const value = deltaValue("temperature", diff, system, decimals);
  if (value > 0)
    return `${formatNumber(value)}° warmer than usual for this hour`;
  if (value < 0)
    return `${formatNumber(-value)}° colder than usual for this hour`;
  return "About average for this hour";
}

// Wind chill from air temperature (°C) and wind (m/s), matching the server's
// feels-like for observations and the normals' hourly samples.
export function feelsLike(
  temp: number | null | undefined,
  wind: number | null | undefined,
): number | null {
  if (temp == null) return null;
  if (wind == null) return temp;
  const windKmh = wind * 3.6;
  if (temp > 10 || windKmh < 4.8) return temp;
  const w = windKmh ** 0.16;
  return 13.12 + 0.6215 * temp - 11.37 * w + 0.3965 * temp * w;
}

export type StatExtra = {
  prefix: string;
  kind: StatKind;
  value: number;
  suffix?: string;
};

export type NormalStat = {
  label: string;
  kind: StatKind;
  value: number;
  normal: number;
  delta: number | null;
  extra?: StatExtra;
};

// The card's summary row: today's forecast high and low, and the observed
// feels-like and wind, each against its normal.
export function normalsSummaryStats(
  normals: DailyClimateNormalsResponse,
  current: CurrentConditions,
  todayForecast: DailyForecast | undefined,
): NormalStat[] {
  const today = normals.today;
  return compact([
    stat("High", "temperature", todayForecast?.high, today.temp_high),
    stat("Low", "temperature", todayForecast?.low, today.temp_low),
    stat(
      "Feels like",
      "temperature",
      current.feels_like,
      today.feels_like_now_normal ?? today.feels_like_avg,
    ),
    stat(
      "Wind",
      "speed",
      current.wind_speed,
      today.wind_now_normal ?? today.wind_avg,
    ),
  ]);
}

// The detail grid, in row pairs: mean/feels-like mean, high/low, feels-like
// high/low, wind/gusts, humidity/precipitation, month to date, then snow when
// there is any.
export function normalsDetailStats(
  normals: DailyClimateNormalsResponse,
  current: CurrentConditions,
  forecast: DailyForecast | undefined,
): NormalStat[] {
  const today = normals.today;
  const precip = normals.precipitation;
  const stats: (NormalStat | null)[] = [
    stat("Mean", "temperature", forecast?.temperature_avg, today.temp_avg),
    stat(
      "Feels like mean",
      "temperature",
      feelsLike(forecast?.temperature_avg, forecast?.wind_speed_avg),
      today.feels_like_avg,
    ),
    stat("High", "temperature", forecast?.high, today.temp_high),
    stat("Low", "temperature", forecast?.low, today.temp_low),
    stat(
      "Feels like high",
      "temperature",
      feelsLike(forecast?.high, forecast?.wind_speed_avg),
      today.feels_like_high,
    ),
    stat(
      "Feels like low",
      "temperature",
      feelsLike(forecast?.low, forecast?.wind_speed_avg),
      today.feels_like_low,
    ),
    stat(
      "Wind",
      "speed",
      current.wind_speed,
      today.wind_now_normal ?? today.wind_avg,
    ),
    stat("Gusts", "speed", forecast?.hourly_maximum_gust_max, today.wind_gust),
    stat(
      "Humidity",
      "percent",
      current.humidity,
      today.humidity_now_normal ?? today.humidity_avg,
    ),
  ];

  if (precip?.today_observed_mm != null) {
    const item = stat(
      "Precip today",
      "precipitation",
      precip.today_observed_mm,
      precip.today_normal_mm ?? today.precip_mm,
    );
    if (item && forecast?.precipitation_mm != null) {
      item.extra = {
        prefix: "forecast",
        kind: "precipitation",
        value: forecast.precipitation_mm,
      };
    }
    stats.push(item);
  } else {
    const item = stat(
      "Precip",
      "precipitation",
      forecast?.precipitation_mm,
      today.precip_mm,
    );
    if (item && today.precip_days_pct != null) {
      item.extra = {
        prefix: "rain on",
        kind: "percent",
        value: today.precip_days_pct,
        suffix: " of days",
      };
    }
    stats.push(item);
  }

  if (precip) {
    const item = stat(
      "Month to date",
      "precipitation",
      precip.month_to_date_observed_mm,
      precip.month_to_date_normal_mm,
    );
    if (item && precip.month_normal_mm != null) {
      item.extra = {
        prefix: "full month",
        kind: "precipitation",
        value: precip.month_normal_mm,
      };
    }
    stats.push(item);
  }

  if (
    today.snow_cm != null &&
    (today.snow_cm >= 0.5 || (current.snow_depth ?? 0) > 0)
  ) {
    stats.push(stat("Snow", "snow", current.snow_depth, today.snow_cm));
  }

  return compact(stats);
}

function stat(
  label: string,
  kind: StatKind,
  current: number | null | undefined,
  normal: number | null | undefined,
): NormalStat | null {
  if (normal == null) return null;
  if (current == null)
    return { label, kind, value: normal, normal, delta: null };
  return { label, kind, value: current, normal, delta: current - normal };
}

function compact(stats: (NormalStat | null)[]): NormalStat[] {
  return stats.filter((item): item is NormalStat => item != null);
}

// The daily forecast entry for the local calendar date, falling back to the
// first entry (the API lists today first).
export function todayForecast(
  daily: readonly DailyForecast[],
  timeZone: string,
  now: Date,
): DailyForecast | undefined {
  const { date } = localDate(now, timeZone);
  return daily.find((day) => day.date === date) ?? daily[0];
}
