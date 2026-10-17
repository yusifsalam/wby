import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { FrameSet } from "./mapOverlay";
import {
  createRadarFrames,
  precipitationRGBA,
  precipitationSteppedRGBA,
  rampColor,
} from "./radarFrames";

describe("rampColor", () => {
  const stops: [number, [number, number, number]][] = [
    [0, [0, 0, 0]],
    [10, [100, 200, 50]],
  ];

  it("clamps outside the stops and interpolates between them", () => {
    expect(rampColor(stops, -5)).toEqual([0, 0, 0]);
    expect(rampColor(stops, 15)).toEqual([100, 200, 50]);
    expect(rampColor(stops, 5)).toEqual([50, 100, 25]);
  });
});

describe("precipitation ramps", () => {
  it("keeps dry cells transparent", () => {
    expect(precipitationRGBA(0.05)[3]).toBe(0);
    expect(precipitationSteppedRGBA(0.05)).toEqual([0, 0, 0, 0]);
  });

  it("reaches full alpha at 1 mm/h on the smooth ramp", () => {
    expect(precipitationRGBA(1)).toEqual([63, 160, 255, 255]);
  });

  it("buckets into FMI's stepped classes", () => {
    expect(precipitationSteppedRGBA(0.4)).toEqual([134, 190, 243, 179]);
    expect(precipitationSteppedRGBA(40)).toEqual([241, 78, 251, 179]);
  });
});

describe("createRadarFrames", () => {
  const NOW = Date.UTC(2026, 9, 17, 12, 0);
  const frames: FrameSet = {
    times: [
      new Date(NOW - 300_000).toISOString(),
      new Date(NOW).toISOString(),
      new Date(NOW + 300_000).toISOString(),
    ],
    nowIndex: 1,
    stepSeconds: 300,
  };
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    fetchMock = vi.fn(async () => new Response(null, { status: 404 }));
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  const requested = () =>
    fetchMock.mock.calls.map(([url]) => new URL(String(url), "http://x"));

  it("reads past frames from the observation grid and falls back to the PNG", async () => {
    const radar = createRadarFrames(() => "smooth");
    const frame = await radar.frameAt(frames, 0);
    const [req] = requested();
    expect(req.pathname).toBe("/api/map/precipitation-observed");
    expect(req.searchParams.get("time")).toBe(frames.times[0]);
    const png = new URL(frame.url, "http://x");
    expect(png.pathname).toBe("/api/map/precipitation");
    expect(png.searchParams.get("time")).toBe(frames.times[0]);
  });

  it("requests the live PNG without a time for the now frame", async () => {
    const radar = createRadarFrames(() => "smooth");
    const frame = await radar.frameAt(frames, frames.nowIndex);
    expect(requested()[0].pathname).toBe("/api/map/precipitation-observed");
    expect(new URL(frame.url, "http://x").searchParams.has("time")).toBe(false);
  });

  it("memoizes observation misses per ramp", async () => {
    let ramp: "smooth" | "stepped" = "smooth";
    const radar = createRadarFrames(() => ramp);
    await radar.frameAt(frames, 0);
    await radar.frameAt(frames, 0);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    ramp = "stepped";
    await radar.frameAt(frames, 0);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("reads future frames from the nowcast grid and expires them", async () => {
    const radar = createRadarFrames(() => "smooth");
    await radar.frameAt(frames, 2);
    await radar.frameAt(frames, 2);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(requested()[0].pathname).toBe("/api/map/precipitation-nowcast");
    vi.setSystemTime(NOW + 61_000);
    await radar.frameAt(frames, 2);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(requested()[1].pathname).toBe("/api/map/precipitation-nowcast");
  });

  it("falls back to the PNG on an upstream error", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    fetchMock.mockImplementation(
      async () => new Response(null, { status: 502 }),
    );
    const radar = createRadarFrames(() => "smooth");
    const frame = await radar.frameAt(frames, 2);
    expect(new URL(frame.url, "http://x").pathname).toBe(
      "/api/map/precipitation",
    );
    expect(console.error).toHaveBeenCalled();
  });
});
