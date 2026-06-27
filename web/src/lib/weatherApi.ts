import type { City } from "./cities";
import { createSignedHeaders } from "./apiSignature";

export type WebConfig = {
  apiBaseUrl: string;
  clientId: string;
  clientSecret: string;
};

export type StationInfo = {
  name: string;
  distance_km: number;
};

export type CurrentConditions = {
  temperature?: number | null;
  feels_like?: number | null;
  wind_speed?: number | null;
  wind_gust?: number | null;
  wind_direction?: number | null;
  humidity?: number | null;
  dew_point?: number | null;
  pressure?: number | null;
  precipitation_1h?: number | null;
  precipitation_intensity?: number | null;
  snow_depth?: number | null;
  visibility?: number | null;
  cloud_cover?: number | null;
  weather_code?: number | null;
  observed_at: string;
};

export type HourlyForecast = {
  time: string;
  temperature?: number | null;
  wind_speed?: number | null;
  wind_direction?: number | null;
  humidity?: number | null;
  precipitation_1h?: number | null;
  symbol?: string | null;
  uv_cumulated?: number | null;
};

export type DailyForecast = {
  date: string;
  high?: number | null;
  low?: number | null;
  temperature_avg?: number | null;
  symbol?: string | null;
  wind_speed_avg?: number | null;
  wind_direction_avg?: number | null;
  humidity_avg?: number | null;
  precipitation_mm?: number | null;
  precipitation_1h_sum?: number | null;
  uv_index_avg?: number | null;
};

export type WeatherResponse = {
  station: StationInfo;
  current: CurrentConditions;
  hourly_forecast: HourlyForecast[];
  daily_forecast: DailyForecast[];
  timezone: string;
};

export const LEADERBOARD_TIMEFRAMES = ["now", "1h", "24h", "3d", "7d"] as const;
export type LeaderboardTimeframe = (typeof LEADERBOARD_TIMEFRAMES)[number];

export type LeaderboardEntry = {
  type: string;
  station_name: string;
  lat: number;
  lon: number;
  value: number;
  unit: string;
  distance_km: number;
  observed_at: string;
};

export type LeaderboardResponse = {
  timeframe: string;
  leaderboard: LeaderboardEntry[];
};

type SignedGetInput = {
  config: WebConfig;
  path: string;
  params: Record<string, string>;
  timestamp: string;
  fetchImpl: typeof fetch;
};

async function signedGet<T>({ config, path, params, timestamp, fetchImpl }: SignedGetInput): Promise<T> {
  const url = new URL(path, config.apiBaseUrl);
  for (const [key, value] of Object.entries(params)) {
    url.searchParams.set(key, value);
  }

  const headers = createSignedHeaders({
    clientId: config.clientId,
    clientSecret: config.clientSecret,
    method: "GET",
    path: url.pathname,
    rawQuery: url.searchParams.toString(),
    timestamp,
  });

  const response = await fetchImpl(url, { method: "GET", headers });
  if (!response.ok) {
    throw new Error(`Weather API failed with ${response.status}: ${await errorMessage(response)}`);
  }

  return (await response.json()) as T;
}

type FetchWeatherInput = {
  city: City;
  config: WebConfig;
  timestamp?: string;
  fetchImpl?: typeof fetch;
};

export async function fetchWeatherForCity({
  city,
  config,
  timestamp = String(Math.floor(Date.now() / 1000)),
  fetchImpl = fetch,
}: FetchWeatherInput): Promise<WeatherResponse> {
  return signedGet<WeatherResponse>({
    config,
    path: "/v1/weather",
    params: {
      lat: formatCoordinate(city.latitude),
      lon: formatCoordinate(city.longitude),
    },
    timestamp,
    fetchImpl,
  });
}

type FetchLeaderboardInput = {
  config: WebConfig;
  timeframe: LeaderboardTimeframe;
  lat: number;
  lon: number;
  timestamp?: string;
  fetchImpl?: typeof fetch;
};

export async function fetchLeaderboard({
  config,
  timeframe,
  lat,
  lon,
  timestamp = String(Math.floor(Date.now() / 1000)),
  fetchImpl = fetch,
}: FetchLeaderboardInput): Promise<LeaderboardResponse> {
  return signedGet<LeaderboardResponse>({
    config,
    path: "/v1/leaderboard",
    params: {
      lat: formatCoordinate(lat),
      lon: formatCoordinate(lon),
      timeframe,
    },
    timestamp,
    fetchImpl,
  });
}

export function cacheControlHeader({
  ttlSeconds,
  staleSeconds,
}: {
  ttlSeconds: number;
  staleSeconds: number;
}): string {
  return `public, max-age=${ttlSeconds}, stale-while-revalidate=${staleSeconds}`;
}

// In `astro dev` we never want the browser to cache pages, so edits show up on
// reload. The production build keeps the public stale-while-revalidate policy.
export function pageCacheControl(policy: { ttlSeconds: number; staleSeconds: number }): string {
  return import.meta.env.DEV ? "no-store" : cacheControlHeader(policy);
}

function formatCoordinate(value: number): string {
  return Number.isInteger(value) ? String(value) : String(value);
}

async function errorMessage(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as { error?: string };
    if (body.error) {
      return body.error;
    }
  } catch {
    // Fall through to the HTTP status text.
  }
  return response.statusText || "upstream error";
}
