import { describe, expect, it } from "vitest";
import type { DailyClimateNormalsResponse } from "./weatherApi";
import { isNormalsForToday } from "./weatherPageData";

const normalsFor = (month: number, day: number) =>
  ({ today: { month, day } }) as DailyClimateNormalsResponse;

describe("isNormalsForToday", () => {
  it("rejects a response fetched before local midnight", () => {
    const fetchedOn14th = normalsFor(11, 14);
    expect(
      isNormalsForToday(fetchedOn14th, new Date("2026-11-14T21:50:00Z")),
    ).toBe(true);
    expect(
      isNormalsForToday(fetchedOn14th, new Date("2026-11-14T22:04:00Z")),
    ).toBe(false);
  });

  it("follows the month across its last day", () => {
    expect(
      isNormalsForToday(normalsFor(11, 30), new Date("2026-11-30T22:30:00Z")),
    ).toBe(false);
    expect(
      isNormalsForToday(normalsFor(12, 1), new Date("2026-11-30T22:30:00Z")),
    ).toBe(true);
  });

  it("keeps a cached absence of normals", () => {
    expect(isNormalsForToday(null, new Date("2026-11-14T22:04:00Z"))).toBe(
      true,
    );
  });
});
