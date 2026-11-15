import { describe, expect, it } from "vitest";
import {
  fallbackFrameSet,
  formatFrameOffset,
  normalizeFrameSet,
} from "./mapOverlay";

describe("fallbackFrameSet", () => {
  it("builds a ±1h precipitation window at 5-minute steps", () => {
    const set = fallbackFrameSet("precipitation", Date.UTC(2026, 7, 22, 12, 3));
    expect(set.times).toHaveLength(25);
    expect(set.nowIndex).toBe(12);
    expect(set.stepSeconds).toBe(300);
    expect(set.times[12]).toBe("2026-08-22T12:00:00.000Z");
    expect(set.times[0]).toBe("2026-08-22T11:00:00.000Z");
  });

  it("builds a now→+48h temperature window at hourly steps", () => {
    const set = fallbackFrameSet("temperature", Date.UTC(2026, 7, 22, 12, 30));
    expect(set.times).toHaveLength(49);
    expect(set.nowIndex).toBe(0);
    expect(set.times[0]).toBe("2026-08-22T12:00:00.000Z");
  });

  it("builds a now→+24h forecast window at hourly steps", () => {
    const set = fallbackFrameSet(
      "precipitation12h",
      Date.UTC(2026, 7, 22, 12, 30),
    );
    expect(set.times).toHaveLength(25);
    expect(set.nowIndex).toBe(0);
    expect(set.stepSeconds).toBe(3600);
    expect(set.times[0]).toBe("2026-08-22T12:00:00.000Z");
    expect(set.times[24]).toBe("2026-08-23T12:00:00.000Z");
  });
});

describe("normalizeFrameSet", () => {
  it("maps the snake_case server shape", () => {
    expect(
      normalizeFrameSet(
        { times: ["a", "b"], now_index: 1, step_seconds: 300 },
        "precipitation",
      ),
    ).toEqual({ times: ["a", "b"], nowIndex: 1, stepSeconds: 300 });
  });

  it("falls back when fields are missing", () => {
    const set = normalizeFrameSet({ times: ["a"] }, "precipitation");
    expect(set.times).toHaveLength(25);
  });
});

describe("formatFrameOffset", () => {
  const frames = { times: Array(25).fill(""), nowIndex: 12, stepSeconds: 300 };
  it("labels now, minutes, and whole hours", () => {
    expect(formatFrameOffset(frames, 12)).toBe("Now");
    expect(formatFrameOffset(frames, 13)).toBe("+5m");
    expect(formatFrameOffset(frames, 0)).toBe("−1h");
  });
});
