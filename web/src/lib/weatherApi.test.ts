import { describe, expect, it } from "vitest";
import { cities } from "./cities";
import { cacheControlHeader, fetchWeatherForCity } from "./weatherApi";

const sampleWeather = {
  station: { name: "Helsinki Kaisaniemi", distance_km: 1.2 },
  current: {
    temperature: 4.4,
    feels_like: 2.1,
    wind_speed: 3.2,
    wind_gust: 5.4,
    wind_direction: 220,
    humidity: 82,
    dew_point: 1.4,
    pressure: 1004,
    precipitation_1h: 0.2,
    visibility: 15000,
    cloud_cover: 7,
    observed_at: "2026-01-27T10:00:00Z",
  },
  hourly_forecast: [],
  daily_forecast: [],
  timezone: "Europe/Helsinki",
};

describe("fetchWeatherForCity", () => {
  it("fetches a city from the signed Go weather endpoint", async () => {
    const calls: Array<{ url: string; headers: Headers }> = [];
    const fetchImpl = async (url: string | URL | Request, init?: RequestInit) => {
      calls.push({
        url: String(url),
        headers: new Headers(init?.headers),
      });
      return new Response(JSON.stringify(sampleWeather), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    };

    const weather = await fetchWeatherForCity({
      city: cities[0],
      config: {
        apiBaseUrl: "http://server:8080",
        clientId: "web",
        clientSecret: "test-secret",
      },
      timestamp: "1769500800",
      fetchImpl,
    });

    expect(weather.current.temperature).toBe(4.4);
    expect(calls).toHaveLength(1);
    expect(calls[0].url).toBe("http://server:8080/v1/weather?lat=60.1699&lon=24.9384");
    expect(calls[0].headers.get("X-Client-ID")).toBe("web");
    expect(calls[0].headers.get("X-Timestamp")).toBe("1769500800");
    expect(calls[0].headers.get("X-Signature")).toBe(
      "2dd7fac401190a36426d6962abbc92d59ba892b02429c044497d3d3e552670b0",
    );
  });

  it("throws a useful error when the upstream API fails", async () => {
    await expect(
      fetchWeatherForCity({
        city: cities[0],
        config: {
          apiBaseUrl: "http://server:8080",
          clientId: "web",
          clientSecret: "test-secret",
        },
        timestamp: "1769500800",
        fetchImpl: async () =>
          new Response(JSON.stringify({ error: "no weather coverage for this location" }), {
            status: 404,
            headers: { "Content-Type": "application/json" },
          }),
      }),
    ).rejects.toThrow("Weather API failed with 404: no weather coverage for this location");
  });
});

describe("cacheControlHeader", () => {
  it("formats public stale-while-revalidate cache policy", () => {
    expect(cacheControlHeader({ ttlSeconds: 900, staleSeconds: 3600 })).toBe(
      "public, max-age=900, stale-while-revalidate=3600",
    );
  });
});
