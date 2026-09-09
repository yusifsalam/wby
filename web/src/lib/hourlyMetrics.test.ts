import { describe, expect, it } from "vitest";
import { formatMeasure } from "./formatters";
import {
  DEFAULT_HOURLY_METRICS,
  HOURLY_METRICS,
  parseHourlyMetrics,
  serializeHourlyMetrics,
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
