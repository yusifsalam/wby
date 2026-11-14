import { describe, expect, it } from "vitest";
import { formatTemperature } from "./formatters";
import {
  buildNormalsChart,
  deltaTone,
  formatDelta,
  formatShownDelta,
  formatStat,
  headlineText,
  hourlyNormalAt,
  localDate,
  localizeHourly,
  niceStep,
  normalsDailySeries,
  normalsDetailStats,
  normalsHeadline,
  normalsHourlySeries,
  normalsSeries,
  normalsSummaryStats,
  shownTone,
  tempNormalAtObservation,
  todayForecast,
} from "./normals";
import type { DailyClimateNormalsResponse } from "./weatherApi";

const tz = "Europe/Helsinki";
// 11:30 local on September 9 (EEST, UTC+3).
const now = new Date("2026-09-09T08:30:00Z");

const hourly = Array.from({ length: 24 }, (_, utcHour) => 10 + utcHour);

const daily = Array.from({ length: 30 }, (_, i) => ({
  month: 9,
  day: i + 1,
  temp_avg: 15 - i * 0.2,
  temp_high: 18 - i * 0.2,
  temp_low: 12 - i * 0.2,
}));

const normals: DailyClimateNormalsResponse = {
  station: { name: "Helsinki Kaisaniemi", distance_km: 0.6 },
  period: "1991-2020",
  today: {
    month: 9,
    day: 9,
    temp_avg: 14.3,
    temp_high: 17.7,
    temp_low: 11.3,
    feels_like_avg: 13.6,
    feels_like_high: 17.2,
    feels_like_low: 10.4,
    wind_avg: 4.1,
    wind_gust: 9.8,
    humidity_avg: 78,
    precip_mm: 2.3,
    precip_days_pct: 48,
    snow_cm: 0,
    temp_hourly: hourly,
    temp_hourly_p10: hourly.map((v) => v - 3),
    temp_hourly_p90: hourly.map((v) => v + 3),
    temp_now_normal: 13.9,
    feels_like_now_normal: 13.2,
    wind_now_normal: 4.4,
    humidity_now_normal: 76,
  },
  precipitation: {
    station: { name: "Espoo Tapiola", distance_km: 2.1 },
    today_observed_mm: 0.2,
    today_normal_mm: 2.3,
    month_to_date_observed_mm: 3.1,
    month_to_date_normal_mm: 9.2,
    month_normal_mm: 66,
  },
  daily,
};

const observed = {
  feels_like: 11.5,
  wind_speed: 6,
  humidity: 71,
  observed_at: "2026-09-09T08:20:00Z",
};

const forecast = {
  date: "2026-09-09",
  high: 18.7,
  low: 13.9,
  temperature_avg: 15.8,
  feels_like_high: 16.2,
  feels_like_low: 11.4,
  feels_like_avg: 13.3,
  wind_speed_avg: 5.2,
  precipitation_mm: 0.4,
  hourly_maximum_gust_max: 11.5,
};

describe("localDate", () => {
  it("resolves the local calendar date, hour and offset", () => {
    expect(localDate(now, tz)).toEqual({
      date: "2026-09-09",
      month: 9,
      day: 9,
      hour: 11,
      offsetHours: 3,
    });
    expect(localDate(new Date("2026-01-09T23:30:00Z"), tz)).toMatchObject({
      date: "2026-01-10",
      hour: 1,
      offsetHours: 2,
    });
  });
});

describe("localizeHourly", () => {
  it("rotates a UTC-indexed curve to local hours", () => {
    const local = localizeHourly(hourly, 3);
    expect(local).toHaveLength(24);
    expect(local[3]).toBe(10);
    expect(local[0]).toBe(31);
    expect(local[23]).toBe(30);
    expect(localizeHourly(hourly.slice(0, 12), 3)).toEqual([]);
    expect(localizeHourly(null, 3)).toEqual([]);
  });
});

describe("series", () => {
  it("builds the hourly curve with the current local hour marked", () => {
    const series = normalsHourlySeries(normals, tz, now);
    expect(series?.mode).toBe("hourly");
    expect(series?.todayIndex).toBe(11);
    expect(series?.avgs[3]).toBe(10);
    expect(series?.highs[3]).toBe(13);
    expect(series?.lows[3]).toBe(7);
    expect(series?.labels.map((l) => l.label)).toEqual([
      "00",
      "06",
      "12",
      "18",
    ]);
    expect(normalsSeries(normals, tz, now)?.mode).toBe("hourly");
  });

  it("builds the month's daily curve", () => {
    const series = normalsDailySeries(normals, tz, now);
    expect(series?.mode).toBe("daily");
    expect(series?.avgs).toHaveLength(30);
    expect(series?.todayIndex).toBe(8);
    expect(series?.highs[0]).toBe(18);
    expect(series?.labels).toEqual([
      { index: 0, label: "1" },
      { index: 7, label: "8" },
      { index: 14, label: "15" },
      { index: 21, label: "22" },
      { index: 29, label: "30" },
    ]);
  });

  it("leaves February 29 out of a non-leap year", () => {
    const february = Array.from({ length: 29 }, (_, i) => ({
      month: 2,
      day: i + 1,
      temp_avg: -5,
      temp_high: -2,
      temp_low: -8,
    }));
    const feb = { ...normals, daily: february };
    const common = normalsDailySeries(
      feb,
      tz,
      new Date("2027-02-10T10:00:00Z"),
    );
    expect(common?.avgs).toHaveLength(28);
    expect(common?.labels.at(-1)).toEqual({ index: 27, label: "28" });
    const leap = normalsDailySeries(feb, tz, new Date("2028-02-10T10:00:00Z"));
    expect(leap?.avgs).toHaveLength(29);
  });

  it("falls back from hourly to daily, then to nothing", () => {
    const noHourly = {
      ...normals,
      today: { ...normals.today, temp_hourly: null },
    };
    expect(normalsHourlySeries(noHourly, tz, now)).toBeNull();
    expect(normalsSeries(noHourly, tz, now)?.mode).toBe("daily");
    expect(normalsSeries({ ...noHourly, daily: [] }, tz, now)).toBeNull();
  });
});

describe("buildNormalsChart", () => {
  it("pads the domain, picks nice ticks and places the markers", () => {
    const series = normalsHourlySeries(normals, tz, now);
    if (!series) throw new Error("expected series");
    const chart = buildNormalsChart(series, 20);
    expect(chart.mode).toBe("hourly");
    expect(chart.min).toBe(5);
    expect(chart.max).toBe(38);
    expect(chart.ticks.map((t) => t.value)).toEqual([10, 20, 30]);
    expect(chart.today.x).toBeCloseTo(((11 + 0.5) / 24) * 100);
    expect(chart.today.meanY).toBeCloseTo(((series.avgs[11] - 5) / 33) * 100);
    expect(chart.now).toMatchObject({ side: "right", y: (15 / 33) * 100 });
    expect(chart.forecastHigh).toBeNull();
    expect(chart.linePath.startsWith("M2.08,")).toBe(true);
    expect(chart.bandPath.endsWith("Z")).toBe(true);
    expect(chart.labels[2]).toMatchObject({ label: "12", today: false });
  });

  it("includes forecast markers in the domain", () => {
    const series = normalsDailySeries(normals, tz, now);
    if (!series) throw new Error("expected series");
    const chart = buildNormalsChart(series, null, { high: 25, low: 2 });
    expect(chart.normalMax).toBe(18);
    expect(chart.normalMin).toBeCloseTo(12 - 29 * 0.2);
    expect(chart.max).toBeGreaterThan(25);
    expect(chart.min).toBe(0);
    expect(chart.max).toBe(27);
    expect(chart.now).toBeNull();
    expect(chart.forecastHigh).toMatchObject({
      x: chart.today.x,
      side: "right",
    });
    expect(chart.forecastHigh?.y).toBeCloseTo((25 / 27) * 100);
    expect(chart.forecastLow?.y).toBeCloseTo((2 / 27) * 100);
  });

  it("omits the band and now marker when unavailable", () => {
    const chart = buildNormalsChart(
      {
        mode: "daily",
        avgs: [1, 2, 3],
        highs: [],
        lows: [],
        todayIndex: 2,
        labels: [],
      },
      null,
    );
    expect(chart.bandPath).toBe("");
    expect(chart.now).toBeNull();
    expect(chart.today.highY).toBeNull();
    expect(chart.today.x).toBeCloseTo((2.5 / 3) * 100);
  });
});

describe("niceStep", () => {
  it("rounds up to 1, 2 or 5 times a power of ten", () => {
    expect(niceStep(0.7)).toBe(1);
    expect(niceStep(1.4)).toBe(2);
    expect(niceStep(3.3)).toBe(5);
    expect(niceStep(6.6)).toBe(10);
    expect(niceStep(0)).toBe(1);
  });
});

describe("formatting", () => {
  it("formats values with units at the requested precision", () => {
    expect(formatStat("temperature", 17.66, "metric")).toBe("18°");
    expect(formatStat("temperature", 17.66, "metric", 1)).toBe("17.7°");
    expect(formatStat("temperature", 0, "imperial", 1)).toBe("32.0°");
    expect(formatStat("temperature", -0.3, "metric")).toBe("0°");
    expect(formatStat("temperature", -0.04, "metric", 1)).toBe("0.0°");
    expect(formatStat("temperature", -0.6, "metric")).toBe("-1°");
    expect(formatStat("speed", 4.4, "metric")).toBe("4 m/s");
    expect(formatStat("speed", 4.4, "imperial", 1)).toBe("9.8 mph");
    expect(formatStat("percent", 76.4, "imperial")).toBe("76%");
    expect(formatStat("percent", 76.4, "metric", 1)).toBe("76%");
    expect(formatDelta("percent", 22.8, "metric", 1)).toBe("+23%");
    expect(formatStat("precipitation", 2.3, "metric", 1)).toBe("2.3 mm");
    expect(formatStat("precipitation", 2.3, "imperial", 1)).toBe("0.09 in");
    expect(formatStat("snow", 8, "imperial")).toBe("3.1 in");
  });

  it("formats signed differences on half-degree and whole-unit steps", () => {
    expect(formatDelta("temperature", 1.3, "metric")).toBe("+1.5°");
    expect(formatDelta("temperature", -2.1, "metric")).toBe("−2°");
    expect(formatDelta("temperature", 0.2, "metric")).toBe("±0°");
    expect(formatDelta("temperature", 1, "imperial")).toBe("+2°");
    expect(formatDelta("speed", 1.6, "metric")).toBe("+2 m/s");
    expect(formatDelta("speed", -1, "imperial")).toBe("−2 mph");
  });

  it("uses tenths at one decimal", () => {
    expect(formatDelta("temperature", 1.34, "metric", 1)).toBe("+1.3°");
    expect(formatDelta("temperature", 1, "metric", 1)).toBe("+1°");
    expect(formatDelta("speed", -0.26, "metric", 1)).toBe("−0.3 m/s");
    expect(formatDelta("precipitation", -2.1, "metric", 1)).toBe("−2.1 mm");
    expect(formatDelta("precipitation", -2.1, "imperial", 1)).toBe("−0.08 in");
  });

  it("classifies the tone from the rounded metric difference", () => {
    expect(deltaTone("temperature", 0.3)).toBe("warm");
    expect(deltaTone("temperature", -0.3)).toBe("cold");
    expect(deltaTone("temperature", 0.2)).toBe("even");
    expect(deltaTone("temperature", 0.2, 1)).toBe("warm");
    expect(deltaTone("speed", 0.4)).toBe("even");
  });
});

describe("differences between displayed values", () => {
  it("rounds negative halves the same way as the forecast rows", () => {
    for (const value of [-10.5, -2.5, -0.5, -0.4, 0.5, 2.5]) {
      expect(formatStat("temperature", value, "metric")).toBe(
        formatTemperature(value),
      );
    }
    expect(formatStat("temperature", -2.5, "metric")).toBe("-3°");
  });

  it("never contradicts the two numbers beside it", () => {
    expect(formatStat("temperature", 4.4, "metric")).toBe("4°");
    expect(formatStat("temperature", 3.6, "metric")).toBe("4°");
    expect(formatShownDelta("temperature", 4.4, 3.6, "metric")).toBe("±0°");
    expect(shownTone("temperature", 4.4, 3.6)).toBe("even");

    expect(formatShownDelta("temperature", 4.6, 3.4, "metric")).toBe("+2°");
    expect(formatShownDelta("temperature", 4.4, 3.6, "metric", 1)).toBe(
      "+0.8°",
    );
    expect(formatShownDelta("temperature", 4.4, 3.6, "imperial")).toBe("+2°");
    expect(formatShownDelta("speed", 6, 4.4, "metric")).toBe("+2 m/s");
    expect(formatShownDelta("precipitation", 0.2, 2.3, "imperial", 1)).toBe(
      "−0.08 in",
    );
    expect(shownTone("temperature", 3.4, 4.6)).toBe("cold");
  });
});

describe("normal for the observation time", () => {
  it("interpolates the UTC hourly curve", () => {
    expect(hourlyNormalAt(hourly, new Date("2026-09-09T08:00:00Z"))).toBe(18);
    expect(hourlyNormalAt(hourly, new Date("2026-09-09T08:30:00Z"))).toBe(18.5);
    expect(
      hourlyNormalAt(hourly, new Date("2026-09-09T23:30:00Z")),
    ).toBeCloseTo((33 + 10) / 2);
    expect(hourlyNormalAt(hourly.slice(0, 12), now)).toBeNull();
    expect(hourlyNormalAt(null, now)).toBeNull();
    expect(hourlyNormalAt(hourly, new Date(""))).toBeNull();
  });

  it("uses the observation time, not the time the normals were fetched", () => {
    expect(
      tempNormalAtObservation(normals, {
        observed_at: "2026-09-09T06:15:00Z",
      }),
    ).toBe(16.25);
    expect(
      tempNormalAtObservation(
        { ...normals, today: { ...normals.today, temp_hourly: null } },
        { observed_at: "2026-09-09T06:15:00Z" },
      ),
    ).toBe(13.9);
  });
});

describe("headline", () => {
  it("compares the current temperature with this hour's normal", () => {
    expect(normalsHeadline(16, 13.9)).toEqual({
      current: 16,
      normal: 13.9,
      tone: "warm",
    });
    expect(normalsHeadline(4.4, 3.6)?.tone).toBe("even");
    expect(normalsHeadline(4.4, 3.6, 1)?.tone).toBe("warm");
    expect(normalsHeadline(null, 13.9)).toBeNull();
    expect(normalsHeadline(16, undefined)).toBeNull();
  });

  it("words the difference between the two displayed temperatures", () => {
    const head = { current: 16, normal: 13.9 };
    expect(headlineText(head, "metric")).toBe(
      "2° warmer than usual for this hour",
    );
    expect(headlineText(head, "metric", 1)).toBe(
      "2.1° warmer than usual for this hour",
    );
    expect(headlineText(head, "imperial")).toBe(
      "4° warmer than usual for this hour",
    );
    expect(headlineText({ current: 12.6, normal: 13.9 }, "metric")).toBe(
      "1° colder than usual for this hour",
    );
    expect(headlineText({ current: 4.4, normal: 3.6 }, "metric")).toBe(
      "About average for this hour",
    );
  });
});

describe("summary stats", () => {
  it("compares forecast high/low and observed feels-like/wind with normals", () => {
    const stats = normalsSummaryStats(normals, observed, forecast);
    expect(stats).toEqual([
      {
        label: "High",
        kind: "temperature",
        value: 18.7,
        normal: 17.7,
        delta: 1,
      },
      {
        label: "Low",
        kind: "temperature",
        value: 13.9,
        normal: 11.3,
        delta: 13.9 - 11.3,
      },
      {
        label: "Feels like",
        kind: "temperature",
        value: 11.5,
        normal: 13.2,
        delta: 11.5 - 13.2,
      },
      { label: "Wind", kind: "speed", value: 6, normal: 4.4, delta: 6 - 4.4 },
    ]);
  });

  it("prefers the hourly curve at the observation time over the fetch-time normal", () => {
    const stats = normalsSummaryStats(
      {
        ...normals,
        today: {
          ...normals.today,
          feels_like_hourly: hourly.map((v) => v - 1),
          wind_hourly: hourly.map((v) => v / 10),
        },
      },
      { ...observed, observed_at: "2026-09-09T06:00:00Z" },
      forecast,
    );
    const byLabel = Object.fromEntries(stats.map((s) => [s.label, s]));
    expect(byLabel["Feels like"].normal).toBe(15);
    expect(byLabel.Wind.normal).toBeCloseTo(1.6);
  });

  it("shows a daily mean uncompared when there is no normal for the hour", () => {
    const stats = normalsDetailStats(
      {
        ...normals,
        today: {
          ...normals.today,
          humidity_now_normal: null,
          wind_now_normal: null,
        },
      },
      observed,
      forecast,
    );
    const byLabel = Object.fromEntries(stats.map((s) => [s.label, s]));
    expect(byLabel.Humidity).toEqual({
      label: "Humidity",
      kind: "percent",
      value: 71,
      normal: 78,
      delta: null,
      basis: "dailyMean",
    });
    expect(byLabel.Wind).toMatchObject({
      value: 6,
      normal: 4.1,
      delta: null,
      basis: "dailyMean",
    });
  });

  it("shows the normal alone when the current value is missing", () => {
    const stats = normalsSummaryStats(
      { ...normals, today: { ...normals.today, feels_like_now_normal: null } },
      { observed_at: "2026-09-09T08:20:00Z" },
      undefined,
    );
    expect(stats).toEqual([
      {
        label: "High",
        kind: "temperature",
        value: 17.7,
        normal: 17.7,
        delta: null,
      },
      {
        label: "Low",
        kind: "temperature",
        value: 11.3,
        normal: 11.3,
        delta: null,
      },
      {
        label: "Feels like",
        kind: "temperature",
        value: 13.6,
        normal: 13.6,
        delta: null,
      },
      { label: "Wind", kind: "speed", value: 4.4, normal: 4.4, delta: null },
    ]);
  });
});

describe("detail stats", () => {
  it("lists the full grid with observed precipitation", () => {
    const stats = normalsDetailStats(normals, observed, forecast);
    expect(stats.map((s) => s.label)).toEqual([
      "Mean",
      "Feels like mean",
      "High",
      "Low",
      "Feels like high",
      "Feels like low",
      "Wind",
      "Gusts",
      "Humidity",
      "Precip today",
      "Month to date",
    ]);
    const byLabel = Object.fromEntries(stats.map((s) => [s.label, s]));
    expect(byLabel["Feels like mean"]).toMatchObject({
      value: 13.3,
      normal: 13.6,
    });
    expect(byLabel["Feels like high"]).toMatchObject({
      value: 16.2,
      normal: 17.2,
    });
    expect(byLabel["Feels like low"]).toMatchObject({
      value: 11.4,
      normal: 10.4,
    });
    expect(byLabel.Gusts).toMatchObject({
      kind: "speed",
      value: 11.5,
      normal: 9.8,
    });
    expect(byLabel.Humidity).toMatchObject({
      kind: "percent",
      value: 71,
      normal: 76,
    });
    expect(byLabel["Precip today"]).toMatchObject({
      kind: "precipitation",
      value: 0.2,
      normal: 2.3,
      extra: { prefix: "forecast", kind: "precipitation", value: 0.4 },
    });
    expect(byLabel["Month to date"]).toMatchObject({
      value: 3.1,
      normal: 9.2,
      extra: { prefix: "full month", kind: "precipitation", value: 66 },
    });
  });

  it("uses the forecast precipitation without a gauge and adds snow when present", () => {
    const stats = normalsDetailStats(
      {
        ...normals,
        precipitation: null,
        today: { ...normals.today, snow_cm: 8 },
      },
      { ...observed, snow_depth: 12 },
      forecast,
    );
    const byLabel = Object.fromEntries(stats.map((s) => [s.label, s]));
    expect(byLabel["Precip today"]).toBeUndefined();
    expect(byLabel["Month to date"]).toBeUndefined();
    expect(byLabel.Precip).toMatchObject({
      value: 0.4,
      normal: 2.3,
      extra: {
        prefix: "rain on",
        kind: "percent",
        value: 48,
        suffix: " of days",
      },
    });
    expect(byLabel.Snow).toMatchObject({
      kind: "snow",
      value: 12,
      normal: 8,
      delta: 4,
    });
  });
});

describe("todayForecast", () => {
  it("picks the entry for the local date, else the first", () => {
    const days = [{ date: "2026-09-08" }, { date: "2026-09-09", high: 18 }];
    expect(todayForecast(days, tz, now)?.high).toBe(18);
    expect(todayForecast([{ date: "2026-09-10" }], tz, now)?.date).toBe(
      "2026-09-10",
    );
    expect(todayForecast([], tz, now)).toBeUndefined();
  });
});
