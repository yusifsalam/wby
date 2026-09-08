import { describe, expect, it } from "vitest";
import {
  celsiusToFahrenheit,
  hectopascalsToInchesHg,
  metersPerSecondToMph,
  metersToFeet,
  metersToMiles,
  millimetersToInches,
  parseUnitSystem,
} from "./units";

describe("unit system setting", () => {
  it("defaults to metric for anything but imperial", () => {
    expect(parseUnitSystem(null)).toBe("metric");
    expect(parseUnitSystem(undefined)).toBe("metric");
    expect(parseUnitSystem("")).toBe("metric");
    expect(parseUnitSystem("metric")).toBe("metric");
    expect(parseUnitSystem("bogus")).toBe("metric");
    expect(parseUnitSystem("imperial")).toBe("imperial");
  });
});

describe("unit conversions", () => {
  it("converts metric values to imperial", () => {
    expect(celsiusToFahrenheit(100)).toBeCloseTo(212);
    expect(celsiusToFahrenheit(-40)).toBeCloseTo(-40);
    expect(metersPerSecondToMph(1)).toBeCloseTo(2.237, 3);
    expect(millimetersToInches(25.4)).toBeCloseTo(1);
    expect(hectopascalsToInchesHg(1013.25)).toBeCloseTo(29.92, 2);
    expect(metersToMiles(1609.344)).toBeCloseTo(1);
    expect(metersToFeet(0.3048)).toBeCloseTo(1);
  });
});
