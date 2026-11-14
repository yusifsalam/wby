import { describe, expect, it } from "vitest";
import {
  uvBarHeight,
  uvColor,
  uvDayBars,
  uvDayPoints,
  uvLevel,
  uvOutlook,
  uvPeak,
} from "./uvIndex";

const tz = "Europe/Helsinki";

// 48 hourly points from 2026-09-09T00:00Z, like the API returns.
const points = Array.from({ length: 48 }, (_, i) => {
  const time = new Date(Date.UTC(2026, 8, 9, i)).toISOString();
  const local = (i + 3) % 24;
  const uv =
    local >= 9 && local <= 17 ? Math.max(0, 3 - Math.abs(local - 13)) : 0;
  return { time, uv };
});

describe("uv index", () => {
  it("maps the displayed whole number to WHO bands", () => {
    expect(uvLevel(0)).toBe("Low");
    expect(uvLevel(2.4)).toBe("Low");
    expect(uvLevel(2.6)).toBe("Moderate");
    expect(uvLevel(3)).toBe("Moderate");
    expect(uvLevel(5.4)).toBe("Moderate");
    expect(uvLevel(5.6)).toBe("High");
    expect(uvLevel(6)).toBe("High");
    expect(uvLevel(7.5)).toBe("Very high");
    expect(uvLevel(10.6)).toBe("Extreme");
    expect(uvLevel(8)).toBe("Very high");
    expect(uvLevel(11)).toBe("Extreme");
    expect(uvLevel(14)).toBe("Extreme");
    expect(uvColor(1)).toBe("#70d65c");
    expect(uvColor(12)).toBe("#cc4de6");
  });

  it("finds the first hour of the highest value", () => {
    expect(
      uvPeak([
        { time: "2026-09-09T06:00:00Z", uv: 1 },
        { time: "2026-09-09T09:00:00Z", uv: 3 },
        { time: "2026-09-09T10:00:00Z", uv: 3 },
        { time: "2026-09-09T11:00:00Z", uv: 2 },
      ]),
    ).toEqual({ value: 3, time: "2026-09-09T09:00:00Z" });
    expect(uvPeak([])).toBeNull();
  });

  it("selects the points of a local calendar day", () => {
    const now = new Date("2026-09-09T08:30:00Z");
    const today = uvDayPoints(points, tz, now);
    expect(today).toHaveLength(21);
    expect(today[0].time).toBe("2026-09-09T00:00:00.000Z");
    expect(today[today.length - 1].time).toBe("2026-09-09T20:00:00.000Z");
    const tomorrow = uvDayPoints(points, tz, now, 1);
    expect(tomorrow).toHaveLength(24);
    expect(tomorrow[0].time).toBe("2026-09-09T21:00:00.000Z");
  });

  it("finds tomorrow by calendar date on a 25-hour day", () => {
    const dstPoints = Array.from({ length: 60 }, (_, i) => ({
      time: new Date(Date.UTC(2027, 9, 30, 21 + i)).toISOString(),
      uv: 1,
    }));
    const justAfterMidnight = new Date("2027-10-30T21:30:00Z");
    const tomorrow = uvDayPoints(dstPoints, tz, justAfterMidnight, 1);
    expect(tomorrow[0].time).toBe("2027-10-31T22:00:00.000Z");
    expect(uvDayPoints(dstPoints, tz, justAfterMidnight)).toHaveLength(25);
  });

  it("describes the rest of the day on whole numbers", () => {
    const at = (value: number) => ({ value, time: "2026-06-09T10:00:00Z" });
    expect(uvOutlook(1.2, at(5.3), at(5.3))).toEqual({
      kind: "rising",
      peak: at(5.3),
    });
    expect(uvOutlook(0.1, at(0.3), at(0.3))).toEqual({ kind: "lowAllDay" });
    expect(uvOutlook(0.2, at(4), at(0.4))).toEqual({ kind: "lowRestOfDay" });
    expect(uvOutlook(4.2, at(4.4), at(4.4))).toEqual({
      kind: "highestRestOfDay",
    });
    expect(uvOutlook(0, null, null)).toEqual({ kind: "lowAllDay" });
  });

  it("builds 24 hourly bars marking past and current hours", () => {
    const now = new Date("2026-09-09T08:30:00Z");
    const bars = uvDayBars(points, tz, now);
    expect(bars).toHaveLength(24);
    expect(bars[0]).toEqual({
      hour: 0,
      time: null,
      uv: null,
      past: true,
      current: false,
    });
    expect(bars[3].uv).toBe(0);
    expect(bars[11]).toMatchObject({ hour: 11, past: false, current: true });
    expect(bars[10].past).toBe(true);
    expect(bars[13].uv).toBe(3);
    expect(bars[13].time).toBe("2026-09-09T10:00:00.000Z");
    expect(uvPeak(uvDayPoints(points, tz, now))).toEqual({
      value: 3,
      time: "2026-09-09T10:00:00.000Z",
    });
  });

  it("scales bar heights to 0–11", () => {
    expect(uvBarHeight(0)).toBe(0);
    expect(uvBarHeight(5.5)).toBe(50);
    expect(uvBarHeight(14)).toBe(100);
  });
});
