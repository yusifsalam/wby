import { describe, expect, it } from "vitest";
import {
  FINLAND,
  FINLAND_H,
  FINLAND_W,
  fallbackFrameSet,
  fixedFrame,
  formatFrameOffset,
  frameTimeParam,
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

describe("frameTimeParam / fixedFrame", () => {
  const frames = { times: ["t0", "t1", "t2"], nowIndex: 1, stepSeconds: 300 };

  it("omits the time for the live frame", () => {
    expect(frameTimeParam(frames, 1)).toBeNull();
    expect(fixedFrame("precipitation", null).url).not.toContain("time=");
  });

  it("pins the overlay URL to the fixed Finland extent", () => {
    const { url, corners } = fixedFrame(
      "precipitation",
      frameTimeParam(frames, 2),
    );
    const q = new URL(url, "http://x").searchParams;
    expect(q.get("bbox")).toBe(
      `${FINLAND.w},${FINLAND.s},${FINLAND.e},${FINLAND.n}`,
    );
    expect(q.get("width")).toBe(String(FINLAND_W));
    expect(q.get("height")).toBe(String(FINLAND_H));
    expect(q.get("time")).toBe("t2");
    expect(corners[0]).toEqual([FINLAND.w, FINLAND.n]);
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
