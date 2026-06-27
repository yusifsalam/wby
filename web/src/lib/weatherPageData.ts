import type { City } from "./cities";
import { readWebConfig, type WebRuntimeConfig } from "./config";
import { WeatherCache, type CacheResult } from "./weatherCache";
import { fetchWeatherForCity, type WeatherResponse } from "./weatherApi";

const caches = new Map<string, WeatherCache<WeatherResponse>>();

export type CityWeatherResult = {
  config: WebRuntimeConfig;
  weather: CacheResult<WeatherResponse>;
};

export async function getCityWeather(city: City, env = process.env): Promise<CityWeatherResult> {
  const config = readWebConfig(env);
  const cache = cacheFor(config);
  const weather = await cache.get(city.slug, () =>
    fetchWeatherForCity({
      city,
      config,
    }),
  );

  return { config, weather };
}

function cacheFor(config: WebRuntimeConfig): WeatherCache<WeatherResponse> {
  const key = `${config.ttlSeconds}:${config.staleSeconds}`;
  const existing = caches.get(key);
  if (existing) {
    return existing;
  }

  const cache = new WeatherCache<WeatherResponse>({
    ttlMs: config.ttlSeconds * 1000,
    staleMs: config.staleSeconds * 1000,
  });
  caches.set(key, cache);
  return cache;
}
