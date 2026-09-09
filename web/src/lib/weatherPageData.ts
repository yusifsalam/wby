import { type City, cities } from "./cities";
import { readWebConfig, type WebRuntimeConfig } from "./config";
import {
  type DailyClimateNormalsResponse,
  fetchDailyClimateNormals,
  fetchLeaderboard,
  fetchWeatherForCity,
  type LeaderboardResponse,
  type LeaderboardTimeframe,
  WeatherApiError,
  type WeatherResponse,
} from "./weatherApi";
import { type CacheResult, WeatherCache } from "./weatherCache";

const caches = new Map<string, WeatherCache<WeatherResponse>>();
const normalsCaches = new Map<
  string,
  WeatherCache<DailyClimateNormalsResponse | null>
>();
const leaderboardCaches = new Map<string, WeatherCache<LeaderboardResponse>>();

export type CityWeatherResult = {
  config: WebRuntimeConfig;
  weather: CacheResult<WeatherResponse>;
  normals: DailyClimateNormalsResponse | null;
};

export async function getCityWeather(
  city: City,
  env = process.env,
): Promise<CityWeatherResult> {
  const config = readWebConfig(env);
  const cache = cacheFor(config);
  const [weather, normals] = await Promise.all([
    cache.get(city.slug, () =>
      fetchWeatherForCity({
        city,
        config,
      }),
    ),
    getCityNormals(city, config),
  ]);

  return { config, weather, normals };
}

// Climate normals are optional: a city without a station within range (404)
// is cached as "none", and any other failure just hides the card for this
// render without poisoning the cache.
async function getCityNormals(
  city: City,
  config: WebRuntimeConfig,
): Promise<DailyClimateNormalsResponse | null> {
  const cache = normalsCacheFor(config);
  try {
    const result = await cache.get(city.slug, async () => {
      try {
        return await fetchDailyClimateNormals({ city, config });
      } catch (error) {
        if (error instanceof WeatherApiError && error.status === 404) {
          return null;
        }
        throw error;
      }
    });
    return result.data;
  } catch (error) {
    console.error(
      `climate normals unavailable for ${city.slug}: ${error instanceof Error ? error.message : String(error)}`,
    );
    return null;
  }
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

function normalsCacheFor(
  config: WebRuntimeConfig,
): WeatherCache<DailyClimateNormalsResponse | null> {
  return cacheFromMap(normalsCaches, config);
}

function leaderboardCacheFor(
  config: WebRuntimeConfig,
): WeatherCache<LeaderboardResponse> {
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
