import { describe, expect, it } from "vitest";
import { formatMeasure } from "./formatters";
import {
  DEFAULT_HOURLY_METRICS,
  HOURLY_METRICS,
  parseHourlyMetrics,
  serializeHourlyMetrics,
  upcomingHours,
} from "./hourlyMetrics";

describe("hourly metric settings", () => {
  it("falls back to the defaults when nothing is stored", () => {
    expect(parseHourlyMetrics(null)).toEqual(DEFAULT_HOURLY_METRICS);
    expect(parseHourlyMetrics(undefined)).toEqual(DEFAULT_HOURLY_METRICS);
  });

  it("treats an empty stored value as showing nothing", () => {
    expect(parseHourlyMetrics("")).toEqual([]);
    expect(parseHourlyMetrics("   ")).toEqual([]);
  });

  it("keeps only known keys in canonical order", () => {
    expect(parseHourlyMetrics("cloud bogus  wind")).toEqual(["wind", "cloud"]);
    expect(parseHourlyMetrics("humidity pop direction")).toEqual([
      "direction",
      "pop",
      "humidity",
    ]);
    expect(parseHourlyMetrics("gust gust pressure")).toEqual([
      "gust",
      "pressure",
    ]);
  });

  it("serializes in canonical order and round-trips", () => {
    expect(serializeHourlyMetrics(["cloud", "wind"])).toBe("wind cloud");
    expect(serializeHourlyMetrics([])).toBe("");
    const all = HOURLY_METRICS.map((metric) => metric.key);
    expect(parseHourlyMetrics(serializeHourlyMetrics(all))).toEqual(all);
  });

  it("formats hourly values and placeholders", () => {
    const hour = {
      time: "2026-08-22T12:00:00Z",
      feels_like: -3.4,
      wind_speed: 4.4,
      wind_gust: 11.6,
      wind_direction: 135,
      precipitation_1h: 0.3,
      pop: 35.4,
      humidity: 80.6,
      pressure: 1008.7,
      cloud_cover: 62.2,
      uv_cumulated: 2.6,
    };
    const metric = Object.fromEntries(
      HOURLY_METRICS.map((m) => [
        m.key,
        formatMeasure(m.measure, m.value(hour)),
      ]),
    );
    expect(metric).toEqual({
      feels: "-3°",
      wind: "4 m/s",
      gust: "12 m/s",
      direction: "SE",
      precip: "0.3 mm",
      pop: "35%",
      humidity: "81%",
      pressure: "1009 hPa",
      cloud: "62%",
      uv: "3",
    });
    const imperial = Object.fromEntries(
      HOURLY_METRICS.map((m) => [
        m.key,
        formatMeasure(m.measure, m.value(hour), "imperial"),
      ]),
    );
    expect(imperial).toEqual({
      feels: "26°",
      wind: "10 mph",
      gust: "26 mph",
      direction: "SE",
      precip: "0.01 in",
      pop: "35%",
      humidity: "81%",
      pressure: "29.79 inHg",
      cloud: "62%",
      uv: "3",
    });
    for (const m of HOURLY_METRICS) {
      expect(
        formatMeasure(m.measure, m.value({ time: "2026-08-22T12:00:00Z" })),
      ).toBe("--");
    }
  });
});

describe("upcomingHours", () => {
  const hoursFrom = (start: string, count: number, temperature: number) =>
    Array.from({ length: count }, (_, i) => ({
      time: new Date(Date.parse(start) + i * 3_600_000).toISOString(),
      temperature,
    }));

  it("starts at the hour in progress, dropping elapsed ones", () => {
    const fetched = hoursFrom("2026-11-14T13:00:00Z", 12, 1);
    const shown = upcomingHours(fetched, new Date("2026-11-14T15:10:00Z"), 12);
    expect(shown[0].time).toBe("2026-11-14T15:00:00.000Z");
    expect(shown).toHaveLength(10);
  });

  it("fills the tail from the long window without replacing fresher hours", () => {
    const fetched = hoursFrom("2026-11-14T13:00:00Z", 12, 1);
    const window = hoursFrom("2026-11-13T22:00:00Z", 240, 2);
    const shown = upcomingHours(
      fetched,
      new Date("2026-11-14T15:10:00Z"),
      12,
      window,
    );
    expect(shown).toHaveLength(12);
    expect(shown.map((hour) => hour.temperature)).toEqual([
      1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2,
    ]);
    expect(shown.at(-1)?.time).toBe("2026-11-15T02:00:00.000Z");
  });

  it("returns nothing when every hour has passed", () => {
    const fetched = hoursFrom("2026-11-14T00:00:00Z", 12, 1);
    expect(
      upcomingHours(fetched, new Date("2026-11-15T00:00:00Z"), 12),
    ).toEqual([]);
  });
});
