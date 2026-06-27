import { describe, expect, it } from "vitest";
import {
  dayLabel,
  formatMillimeters,
  formatObservedTime,
  formatPercent,
  formatSpeed,
  formatTemperature,
  formatVisibility,
  hourLabel,
} from "./formatters";

describe("weather value formatters", () => {
  it("formats core metric weather values", () => {
    expect(formatTemperature(4.4)).toBe("4°");
    expect(formatTemperature(null)).toBe("--");
    expect(formatSpeed(3.2)).toBe("3 m/s");
    expect(formatPercent(81.6)).toBe("82%");
    expect(formatMillimeters(0.24)).toBe("0.2 mm");
    expect(formatMillimeters(2.01)).toBe("2 mm");
    expect(formatVisibility(15000)).toBe("15.0 km");
  });

  it("formats times in the weather response timezone", () => {
    expect(formatObservedTime("2026-01-27T10:00:00Z", "Europe/Helsinki")).toBe("12:00");
    expect(hourLabel("2026-01-27T10:00:00Z", "Europe/Helsinki")).toBe("12");
    expect(dayLabel("2026-01-27", "Europe/Helsinki")).toBe("Tue");
  });
});
