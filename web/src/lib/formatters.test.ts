import { describe, expect, it } from "vitest";
import {
  dayLabel,
  formatMeasure,
  formatObservedTime,
  formatPercent,
  formatPrecipitation,
  formatPressure,
  formatRelativeTime,
  formatSpeed,
  formatTemperature,
  formatVisibility,
  formatWindDirection,
  hourLabel,
} from "./formatters";

describe("weather value formatters", () => {
  it("formats core metric weather values", () => {
    expect(formatTemperature(4.4)).toBe("4°");
    expect(formatTemperature(null)).toBe("--");
    expect(formatSpeed(3.2)).toBe("3 m/s");
    expect(formatPercent(81.6)).toBe("82%");
    expect(formatPrecipitation(0.24)).toBe("0.2 mm");
    expect(formatPressure(1013.4)).toBe("1013 hPa");
    expect(formatPressure(null)).toBe("--");
    expect(formatWindDirection(0)).toBe("N");
    expect(formatWindDirection(44)).toBe("NE");
    expect(formatWindDirection(202.5)).toBe("SW");
    expect(formatWindDirection(359)).toBe("N");
    expect(formatWindDirection(-90)).toBe("W");
    expect(formatWindDirection(null)).toBe("--");
    expect(formatPrecipitation(2.01)).toBe("2 mm");
    expect(formatVisibility(15000)).toBe("15.0 km");
    expect(formatVisibility(800)).toBe("800 m");
  });

  it("formats imperial weather values", () => {
    expect(formatTemperature(0, "imperial")).toBe("32°");
    expect(formatTemperature(-40, "imperial")).toBe("-40°");
    expect(formatTemperature(21.3, "imperial")).toBe("70°");
    expect(formatTemperature(null, "imperial")).toBe("--");
    expect(formatSpeed(10, "imperial")).toBe("22 mph");
    expect(formatSpeed(null, "imperial")).toBe("--");
    expect(formatPrecipitation(0, "imperial")).toBe("0 in");
    expect(formatPrecipitation(0.1, "imperial")).toBe("<0.01 in");
    expect(formatPrecipitation(0.3, "imperial")).toBe("0.01 in");
    expect(formatPrecipitation(12.7, "imperial")).toBe("0.50 in");
    expect(formatPrecipitation(25.4, "imperial")).toBe("1 in");
    expect(formatPressure(1013.25, "imperial")).toBe("29.92 inHg");
    expect(formatVisibility(15000, "imperial")).toBe("9.3 mi");
    expect(formatVisibility(800, "imperial")).toBe("2625 ft");
    expect(formatVisibility(null, "imperial")).toBe("--");
  });

  it("dispatches by measure kind", () => {
    expect(formatMeasure("temperature", 10)).toBe("10°");
    expect(formatMeasure("temperature", 10, "imperial")).toBe("50°");
    expect(formatMeasure("speed", 5, "imperial")).toBe("11 mph");
    expect(formatMeasure("precipitation", 2.5, "imperial")).toBe("0.10 in");
    expect(formatMeasure("pressure", 1000, "imperial")).toBe("29.53 inHg");
    expect(formatMeasure("visibility", 3218.7, "imperial")).toBe("2.0 mi");
    expect(formatMeasure("percent", 50, "imperial")).toBe("50%");
    expect(formatMeasure("direction", 90, "imperial")).toBe("E");
    expect(formatMeasure("index", 2.6, "imperial")).toBe("3");
    expect(formatMeasure("index", null)).toBe("--");
    expect(formatMeasure("temperature", null, "imperial")).toBe("--");
  });

  it("formats relative observation times", () => {
    const now = new Date("2026-06-27T12:00:00Z").getTime();
    expect(formatRelativeTime("2026-06-27T11:59:30Z", now)).toBe("just now");
    expect(formatRelativeTime("2026-06-27T11:45:00Z", now)).toBe("15 min ago");
    expect(formatRelativeTime("2026-06-27T09:00:00Z", now)).toBe("3h ago");
    expect(formatRelativeTime("2026-06-24T12:00:00Z", now)).toBe("3d ago");
  });

  it("formats times in the weather response timezone", () => {
    expect(formatObservedTime("2026-01-27T10:00:00Z", "Europe/Helsinki")).toBe(
      "12:00",
    );
    expect(hourLabel("2026-01-27T10:00:00Z", "Europe/Helsinki")).toBe("12");
    expect(dayLabel("2026-01-27", "Europe/Helsinki")).toBe("Tue");
  });
});
