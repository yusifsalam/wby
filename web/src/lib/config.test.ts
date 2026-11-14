import { describe, expect, it } from "vitest";
import { readWebConfig } from "./config";

describe("readWebConfig", () => {
  it("reads required API credentials and cache settings", () => {
    expect(
      readWebConfig({
        WBY_API_BASE_URL: "http://server:8080",
        WBY_API_CLIENT_ID: "web",
        WBY_API_CLIENT_SECRET: "secret",
        WBY_CACHE_TTL_SECONDS: "900",
        WBY_CACHE_STALE_SECONDS: "3600",
      }),
    ).toEqual({
      apiBaseUrl: "http://server:8080",
      clientId: "web",
      clientSecret: "secret",
      ttlSeconds: 900,
      staleSeconds: 3600,
    });
  });

  it("uses cache defaults but requires signing credentials", () => {
    expect(() => readWebConfig({})).toThrow("WBY_API_CLIENT_ID is required");
    expect(() => readWebConfig({ WBY_API_CLIENT_ID: "web" })).toThrow(
      "WBY_API_CLIENT_SECRET is required",
    );

    expect(
      readWebConfig({
        WBY_API_CLIENT_ID: "web",
        WBY_API_CLIENT_SECRET: "secret",
      }),
    ).toMatchObject({
      apiBaseUrl: "http://localhost:8080",
      ttlSeconds: 900,
      staleSeconds: 3600,
    });
  });

  it("rejects values it would otherwise have to guess at", () => {
    const base = { WBY_API_CLIENT_ID: "web", WBY_API_CLIENT_SECRET: "secret" };
    for (const bad of ["15m", "1e3", "0", "-5", "12.5"]) {
      expect(() =>
        readWebConfig({ ...base, WBY_CACHE_TTL_SECONDS: bad }),
      ).toThrow("WBY_CACHE_TTL_SECONDS must be a whole number of seconds");
    }
    expect(() =>
      readWebConfig({ ...base, WBY_API_BASE_URL: "server:8080" }),
    ).toThrow("WBY_API_BASE_URL must be an http or https URL");
    expect(
      readWebConfig({ ...base, WBY_CACHE_TTL_SECONDS: " 60 " }).ttlSeconds,
    ).toBe(60);
  });

  it("lists every problem at once", () => {
    expect(() =>
      readWebConfig({
        WBY_API_CLIENT_ID: "web",
        WBY_CACHE_STALE_SECONDS: "soon",
      }),
    ).toThrow(
      "WBY_API_CLIENT_SECRET is required; WBY_CACHE_STALE_SECONDS must be a whole number of seconds above zero",
    );
  });
});
