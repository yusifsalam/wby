import { describe, expect, it } from "vitest";
import { buildDayChart, groupHoursByLocalDate, HOUR_LABELS, summarizeDay } from "./dayDetail";
import { localHour } from "./formatters";
import type { HourlyForecast } from "./weatherApi";

const tz = "Europe/Helsinki";

function hour(time: string, extra: Partial<HourlyForecast> = {}): HourlyForecast {
  return { time, ...extra };
}

describe("groupHoursByLocalDate", () => {
  it("buckets by the local calendar day, not the UTC one", () => {
    // 21:30 UTC on the 9th is 00:30 on the 10th in Helsinki (UTC+3).
    const groups = groupHoursByLocalDate(
      [hour("2026-09-09T20:00:00Z"), hour("2026-09-09T21:00:00Z"), hour("2026-09-10T05:00:00Z")],
      tz,
    );
    expect([...groups.keys()]).toEqual(["2026-09-09", "2026-09-10"]);
    expect(groups.get("2026-09-10")).toHaveLength(2);
  });
});

describe("summarizeDay", () => {
  it("prefers the daily row and derives the rest from the hours", () => {
    const summary = summarizeDay(
      { date: "2026-09-10", high: 17, low: 13, precipitation_mm: 10.8 },
      [
        hour("2026-09-10T04:00:00Z", { pop: 40, wind_speed: 4, wind_gust: 9, humidity: 80, uv_cumulated: 1 }),
        hour("2026-09-10T05:00:00Z", { pop: 95, wind_speed: 6, wind_gust: 12, humidity: 90, uv_cumulated: 3 }),
      ],
    );
    expect(summary).toEqual({
      high: 17,
      low: 13,
      precipitation: 10.8,
      popMax: 95,
      windAvg: 5,
      gustMax: 12,
      humidityAvg: 85,
      uvMax: 3,
    });
  });

  it("falls back to the hours when the daily row is empty", () => {
    const summary = summarizeDay({ date: "2026-09-10" }, [
      hour("2026-09-10T04:00:00Z", { temperature: 10, precipitation_1h: 0.5 }),
      hour("2026-09-10T05:00:00Z", { temperature: 14, precipitation_1h: 1 }),
    ]);
    expect(summary.high).toBe(14);
    expect(summary.low).toBe(10);
    expect(summary.precipitation).toBe(1.5);
    expect(summary.popMax).toBeNull();
  });
});

describe("buildDayChart", () => {
  const local = (time: string) => localHour(time, tz);

  it("returns null for a single hour", () => {
    expect(buildDayChart([hour("2026-09-10T04:00:00Z", { temperature: 10 })], local)).toBeNull();
  });

  it("omits the temperature curve without temperatures but keeps the histogram", () => {
    const chart = buildDayChart(
      [hour("2026-09-10T04:00:00Z"), hour("2026-09-10T05:00:00Z")],
      local,
    );
    expect(chart?.temperature).toBeNull();
    expect(chart?.precipitation).toEqual({ max: 1, bars: [], ticks: [{ value: 0.5, y: 50 }, { value: 1, y: 100 }] });
  });

  it("anchors every slot to its hour of day so the axis is the same for every day", () => {
    // 03:00–06:00 UTC is 06:00–09:00 in Helsinki; the slots sit at those
    // hours on a 24-hour axis even though only four hours are present.
    const hours = [
      hour("2026-09-10T03:00:00Z", { temperature: 10, feels_like: 8, precipitation_1h: 0 }),
      hour("2026-09-10T04:00:00Z", { temperature: 12, feels_like: 10, precipitation_1h: 2 }),
      hour("2026-09-10T05:00:00Z", { temperature: 14, feels_like: 12, precipitation_1h: 3 }),
      hour("2026-09-10T06:00:00Z", { temperature: 16, precipitation_1h: 0 }),
    ];
    const chart = buildDayChart(hours, local);
    expect(chart).not.toBeNull();
    if (!chart?.temperature) return;
    const slot = (h: number) => ((h + 0.5) / 24) * 100;
    // SVG path coordinates are rounded to two decimals; bar positions are not.
    const at = (h: number) => Math.round(slot(h) * 100) / 100;
    // Axis pads the 8..16 range by one degree each side; 16° sits at the top pad.
    expect(chart.temperature.min).toBe(7);
    expect(chart.temperature.max).toBe(17);
    expect(chart.temperature.linePath).toBe(`M${at(6)},70L${at(7)},50L${at(8)},30L${at(9)},10`);
    expect(chart.temperature.feelsPath).toBe(`M${at(6)},90L${at(7)},70L${at(8)},50`);
    expect(chart.temperature.ticks.map((t) => t.value)).toEqual([10, 15]);
    // Histogram axis rounds 3 mm up to a 4 mm top with 2 mm steps; bars are
    // 70% of a 1/24 slot wide.
    expect(chart.precipitation.max).toBe(4);
    expect(chart.precipitation.bars).toEqual([
      { x: slot(7), width: (100 / 24) * 0.7, height: 50, value: 2 },
      { x: slot(8), width: (100 / 24) * 0.7, height: 75, value: 3 },
    ]);
    expect(chart.precipitation.ticks).toEqual([
      { value: 2, y: 50 },
      { value: 4, y: 100 },
    ]);
    expect(chart.hourLabels).toBe(HOUR_LABELS);
  });

  it("labels the axis every three hours with six-hourly majors", () => {
    expect(HOUR_LABELS.map((t) => t.label)).toEqual(["00", "03", "06", "09", "12", "15", "18", "21"]);
    expect(HOUR_LABELS.filter((t) => t.major).map((t) => t.label)).toEqual(["00", "06", "12", "18"]);
  });
});
