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
});
