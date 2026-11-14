import { describe, expect, it } from "vitest";
import { Cooldown, WeatherCache } from "./weatherCache";

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

  it("reloads an entry the caller no longer finds usable", async () => {
    let calls = 0;
    const cache = new WeatherCache<string>({
      ttlMs: 900_000,
      staleMs: 3_600_000,
      now: () => 1_000,
    });
    const load = async () => {
      calls += 1;
      return `load ${calls}`;
    };

    await cache.get("helsinki", load);
    const kept = await cache.get("helsinki", load, () => true);
    const reloaded = await cache.get(
      "helsinki",
      load,
      (data) => data !== "load 1",
    );
    const after = await cache.get(
      "helsinki",
      load,
      (data) => data !== "load 1",
    );

    expect(kept.data).toBe("load 1");
    expect(reloaded).toMatchObject({ state: "fresh", data: "load 2" });
    expect(after.data).toBe("load 2");
    expect(calls).toBe(2);
  });

  it("shares one load between concurrent requests for a cold key", async () => {
    let calls = 0;
    let release: (value: string) => void = () => {};
    const cache = new WeatherCache<string>({
      ttlMs: 900_000,
      staleMs: 3_600_000,
      now: () => 1_000,
    });
    const load = () => {
      calls += 1;
      return new Promise<string>((resolve) => {
        release = resolve;
      });
    };

    const first = cache.get("helsinki", load);
    const second = cache.get("helsinki", load);
    const other = cache.get("espoo", async () => "espoo");
    release("shared");

    expect((await first).data).toBe("shared");
    expect((await second).data).toBe("shared");
    expect((await other).data).toBe("espoo");
    expect(calls).toBe(1);
  });

  it("does not cache a failed load and lets the next request retry", async () => {
    const cache = new WeatherCache<string>({
      ttlMs: 900_000,
      staleMs: 3_600_000,
      now: () => 1_000,
    });
    const failing = async () => {
      throw new Error("upstream down");
    };

    await expect(cache.get("helsinki", failing)).rejects.toThrow(
      "upstream down",
    );
    const retried = await cache.get("helsinki", async () => "recovered");
    expect(retried.data).toBe("recovered");
  });

  it("reports a failed background refresh and keeps serving the old data", async () => {
    let now = 1_000;
    const failures: string[] = [];
    const cache = new WeatherCache<string>({
      ttlMs: 900_000,
      staleMs: 3_600_000,
      now: () => now,
      onRefreshError: (key, error) =>
        failures.push(`${key}: ${(error as Error).message}`),
    });

    await cache.get("helsinki", async () => "initial");
    now += 1_000_000;
    const stale = await cache.get("helsinki", async () => {
      throw new Error("upstream down");
    });
    await stale.refresh;

    expect(stale).toMatchObject({ state: "stale", data: "initial" });
    expect(failures).toEqual(["helsinki: upstream down"]);
  });
});

describe("Cooldown", () => {
  it("is active for the given time after a failure", () => {
    let now = 0;
    const cooldown = new Cooldown(60_000, () => now);

    expect(cooldown.active("hourly:helsinki")).toBe(false);
    cooldown.start("hourly:helsinki");
    now = 59_999;
    expect(cooldown.active("hourly:helsinki")).toBe(true);
    expect(cooldown.active("hourly:espoo")).toBe(false);
    now = 60_000;
    expect(cooldown.active("hourly:helsinki")).toBe(false);
  });
});
