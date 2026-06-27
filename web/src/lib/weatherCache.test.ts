import { describe, expect, it } from "vitest";
import { WeatherCache } from "./weatherCache";

describe("WeatherCache", () => {
  it("returns fresh cached data without calling the loader again", async () => {
    let now = 1_000;
    let calls = 0;
    const cache = new WeatherCache<string>({
      ttlMs: 900_000,
      staleMs: 3_600_000,
      now: () => now,
    });

    const first = await cache.get("helsinki", async () => {
      calls += 1;
      return "initial";
    });
    now += 60_000;
    const second = await cache.get("helsinki", async () => {
      calls += 1;
      return "new";
    });

    expect(first).toMatchObject({ state: "fresh", data: "initial" });
    expect(second).toMatchObject({ state: "fresh", data: "initial" });
    expect(calls).toBe(1);
  });

  it("serves stale data while one background refresh is in flight", async () => {
    let now = 1_000;
    let calls = 0;
    let resolveRefresh: (value: string) => void = () => {};
    const cache = new WeatherCache<string>({
      ttlMs: 900_000,
      staleMs: 3_600_000,
      now: () => now,
    });

    await cache.get("oulu", async () => "initial");
    now += 901_000;

    const firstStale = await cache.get(
      "oulu",
      () =>
        new Promise<string>((resolve) => {
          calls += 1;
          resolveRefresh = resolve;
        }),
    );
    const secondStale = await cache.get("oulu", async () => {
      calls += 1;
      return "duplicate";
    });

    expect(firstStale).toMatchObject({ state: "stale", data: "initial" });
    expect(secondStale).toMatchObject({ state: "stale", data: "initial" });
    expect(calls).toBe(1);

    resolveRefresh("refreshed");
    await firstStale.refresh;
    const fresh = await cache.get("oulu", async () => "unused");

    expect(fresh).toMatchObject({ state: "fresh", data: "refreshed" });
  });

  it("keeps serving stale data when a background refresh fails", async () => {
    let now = 1_000;
    const cache = new WeatherCache<string>({
      ttlMs: 900_000,
      staleMs: 3_600_000,
      now: () => now,
    });

    await cache.get("turku", async () => "initial");
    now += 901_000;

    const stale = await cache.get("turku", async () => {
      throw new Error("upstream unavailable");
    });
    await stale.refresh;

    expect(stale).toMatchObject({ state: "stale", data: "initial" });
    const stillStale = await cache.get("turku", async () => "unused");
    expect(stillStale).toMatchObject({ state: "stale", data: "initial" });
  });

  it("throws when there is no cached data and the loader fails", async () => {
    const cache = new WeatherCache<string>({
      ttlMs: 900_000,
      staleMs: 3_600_000,
      now: () => 1_000,
    });

    await expect(
      cache.get("missing", async () => {
        throw new Error("upstream unavailable");
      }),
    ).rejects.toThrow("upstream unavailable");
  });
});
