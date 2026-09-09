import { niceStep } from "./normals";
import type { DailyForecast, HourlyForecast } from "./weatherApi";

// Buckets the hourly window by local calendar day (YYYY-MM-DD in timeZone).
export function groupHoursByLocalDate(
  hours: readonly HourlyForecast[],
  timeZone: string,
): Map<string, HourlyForecast[]> {
  const formatter = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  });
  const groups = new Map<string, HourlyForecast[]>();
  for (const hour of hours) {
    const date = formatter.format(new Date(hour.time));
    const bucket = groups.get(date);
    if (bucket) {
      bucket.push(hour);
    } else {
      groups.set(date, [hour]);
    }
  }
  return groups;
}

export type DaySummary = {
  high: number | null;
  low: number | null;
  precipitation: number | null;
  popMax: number | null;
  windAvg: number | null;
  gustMax: number | null;
  humidityAvg: number | null;
  cloudCoverAvg: number | null;
};

// High and low come from the daily row the user clicked so the sheet repeats
// the numbers they saw. Precipitation is the total over the hours shown, so
// it matches the histogram even for today, whose daily row only covers the
// hours after the forecast was fetched. The rest is derived from the hours.
export function summarizeDay(
  day: DailyForecast,
  hours: readonly HourlyForecast[],
): DaySummary {
  const temps = defined(hours.map((h) => h.temperature));
  return {
    high: day.high ?? maxOf(temps),
    low: day.low ?? minOf(temps),
    precipitation:
      sumOf(defined(hours.map((h) => h.precipitation_1h))) ??
      day.precipitation_mm ??
      day.precipitation_1h_sum ??
      null,
    popMax: maxOf(defined(hours.map((h) => h.pop))),
    windAvg:
      day.wind_speed_avg ?? avgOf(defined(hours.map((h) => h.wind_speed))),
    gustMax:
      day.hourly_maximum_gust_max ??
      maxOf(defined(hours.map((h) => h.wind_gust))),
    humidityAvg:
      day.humidity_avg ?? avgOf(defined(hours.map((h) => h.humidity))),
    cloudCoverAvg:
      day.total_cloud_cover_avg ??
      avgOf(defined(hours.map((h) => h.cloud_cover))),
  };
}

export type AxisTick = { value: number; y: number };

export type TemperatureChart = {
  min: number;
  max: number;
  linePath: string;
  feelsPath: string;
  ticks: AxisTick[];
};

export type PrecipitationChart = {
  max: number;
  bars: { x: number; width: number; height: number; value: number }[];
  ticks: AxisTick[];
};

export type DayChart = {
  temperature: TemperatureChart | null;
  precipitation: PrecipitationChart;
  hourLabels: { x: number; label: string; major: boolean }[];
};

// The hour axis is always the full local day, so a chart for today shows
// the elapsed hours as empty space and every day lines up the same way.
export const HOUR_LABELS: DayChart["hourLabels"] = Array.from(
  { length: 8 },
  (_, i) => {
    const hour = i * 3;
    return {
      x: slotX(hour),
      label: String(hour).padStart(2, "0"),
      major: hour % 6 === 0,
    };
  },
);

function slotX(hour: number): number {
  return ((hour + 0.5) / 24) * 100;
}

// Temperature/feels-like curves and a precipitation histogram for one day,
// each in percentages of its own plot box and sharing the hour axis.
// `localHour` gives an entry's hour of day (0–23) in the place's timezone.
export function buildDayChart(
  hours: readonly HourlyForecast[],
  localHour: (time: string) => number,
): DayChart | null {
  if (hours.length < 2) return null;
  const x = (index: number) => slotX(localHour(hours[index].time));
  return {
    temperature: temperatureChart(hours, x),
    precipitation: precipitationChart(hours, x),
    hourLabels: HOUR_LABELS,
  };
}

function temperatureChart(
  hours: readonly HourlyForecast[],
  x: (index: number) => number,
): TemperatureChart | null {
  const temps = defined(hours.map((h) => h.temperature));
  if (temps.length < 2) return null;
  const values = [...temps, ...defined(hours.map((h) => h.feels_like))];
  const min = Math.floor(Math.min(...values)) - 1;
  const max = Math.ceil(Math.max(...values)) + 1;
  const y = (value: number) => ((value - min) / (max - min)) * 100;

  const path = (pick: (hour: HourlyForecast) => number | null | undefined) => {
    const parts: string[] = [];
    hours.forEach((hour, i) => {
      const value = pick(hour);
      if (value == null) return;
      parts.push(
        `${parts.length === 0 ? "M" : "L"}${fmt(x(i))},${fmt(100 - y(value))}`,
      );
    });
    return parts.length > 1 ? parts.join("") : "";
  };

  return {
    min,
    max,
    linePath: path((h) => h.temperature),
    feelsPath: path((h) => h.feels_like),
    ticks: axisTicks(min, max, 4, y),
  };
}

// The histogram always renders, so a dry day reads as an empty 0–1 mm plot
// rather than a missing one.
function precipitationChart(
  hours: readonly HourlyForecast[],
  x: (index: number) => number,
): PrecipitationChart {
  const precip = hours.map((h) => h.precipitation_1h ?? 0);
  const step = niceStep(Math.max(...precip, 1) / 2);
  const max = Math.max(1, Math.ceil(Math.max(...precip, 0) / step) * step);
  const y = (value: number) => (value / max) * 100;
  const width = (100 / 24) * 0.7;
  const bars: PrecipitationChart["bars"] = [];
  precip.forEach((value, i) => {
    if (value <= 0) return;
    bars.push({ x: x(i), width, height: y(value), value });
  });
  return { max, bars, ticks: axisTicks(step, max, 0, y, step) };
}

function axisTicks(
  min: number,
  max: number,
  divisions: number,
  y: (value: number) => number,
  step = niceStep((max - min) / divisions),
): AxisTick[] {
  const ticks: AxisTick[] = [];
  for (let value = Math.ceil(min / step) * step; value <= max; value += step) {
    ticks.push({ value, y: y(value) });
  }
  return ticks;
}

function defined(values: readonly (number | null | undefined)[]): number[] {
  return values.filter((value): value is number => value != null);
}

function maxOf(values: number[]): number | null {
  return values.length > 0 ? Math.max(...values) : null;
}

function minOf(values: number[]): number | null {
  return values.length > 0 ? Math.min(...values) : null;
}

function sumOf(values: number[]): number | null {
  return values.length > 0 ? values.reduce((a, b) => a + b, 0) : null;
}

function avgOf(values: number[]): number | null {
  const sum = sumOf(values);
  return sum == null ? null : sum / values.length;
}

function fmt(value: number): string {
  return String(Math.round(value * 100) / 100);
}
