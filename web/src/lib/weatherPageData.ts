import { cities, type City } from "./cities";
import { readWebConfig, type WebRuntimeConfig } from "./config";
import { WeatherCache, type CacheResult } from "./weatherCache";
import {
  fetchLeaderboard,
  fetchWeatherForCity,
  type LeaderboardResponse,
  type LeaderboardTimeframe,
  type WeatherResponse,
} from "./weatherApi";

const caches = new Map<string, WeatherCache<WeatherResponse>>();
const leaderboardCaches = new Map<string, WeatherCache<LeaderboardResponse>>();

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

export type LeaderboardResult = {
  config: WebRuntimeConfig;
  timeframe: LeaderboardTimeframe;
  leaderboard: CacheResult<LeaderboardResponse>;
};

// The leaderboard ranks all Finnish stations, so we anchor the request to a
// fixed reference point (Helsinki) — lat/lon only drives the per-entry
// distance_km the backend returns, not the ranking.
const leaderboardReference = cities[0];

export async function getLeaderboard(
  timeframe: LeaderboardTimeframe,
  env = process.env,
): Promise<LeaderboardResult> {
  const config = readWebConfig(env);
  const cache = leaderboardCacheFor(config);
  const leaderboard = await cache.get(timeframe, () =>
    fetchLeaderboard({
      config,
      timeframe,
      lat: leaderboardReference.latitude,
      lon: leaderboardReference.longitude,
    }),
  );

  return { config, timeframe, leaderboard };
}

function cacheFor(config: WebRuntimeConfig): WeatherCache<WeatherResponse> {
  return cacheFromMap(caches, config);
}

function leaderboardCacheFor(config: WebRuntimeConfig): WeatherCache<LeaderboardResponse> {
  return cacheFromMap(leaderboardCaches, config);
}

function cacheFromMap<T>(
  store: Map<string, WeatherCache<T>>,
  config: WebRuntimeConfig,
): WeatherCache<T> {
  const key = `${config.ttlSeconds}:${config.staleSeconds}`;
  const existing = store.get(key);
  if (existing) {
    return existing;
  }

  const cache = new WeatherCache<T>({
    ttlMs: config.ttlSeconds * 1000,
    staleMs: config.staleSeconds * 1000,
  });
  store.set(key, cache);
  return cache;
}
