import { createSignedHeaders } from "./apiSignature";
import type { City } from "./cities";

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
  feels_like?: number | null;
  wind_speed?: number | null;
  wind_direction?: number | null;
  humidity?: number | null;
  precipitation_1h?: number | null;
  symbol?: string | null;
  uv_cumulated?: number | null;
  wind_gust?: number | null;
  pressure?: number | null;
  cloud_cover?: number | null;
  pop?: number | null;
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
  hourly_maximum_gust_max?: number | null;
  uv_index_avg?: number | null;
};

// Hourly UV index from the local midnight through the next day.
export type UVPoint = {
  time: string;
  uv: number;
};

export type WeatherResponse = {
  station: StationInfo;
  current: CurrentConditions;
  hourly_forecast: HourlyForecast[];
  daily_forecast: DailyForecast[];
  uv_forecast?: UVPoint[];
  timezone: string;
};

// One calendar day's 1991–2020 normals from /v1/climate-normals/daily.
export type DailyNormal = {
  month: number;
  day: number;
  temp_avg?: number | null;
  temp_high?: number | null;
  temp_low?: number | null;
  feels_like_avg?: number | null;
  feels_like_high?: number | null;
  feels_like_low?: number | null;
  wind_avg?: number | null;
  wind_gust?: number | null;
  humidity_avg?: number | null;
  precip_mm?: number | null;
  precip_days_pct?: number | null;
  snow_cm?: number | null;
};

// Today's row plus 24-value curves indexed by UTC hour and the normal for the
// current hour.
export type DailyNormalToday = DailyNormal & {
  temp_hourly?: number[] | null;
  temp_hourly_p10?: number[] | null;
  temp_hourly_p90?: number[] | null;
  feels_like_hourly?: number[] | null;
  wind_hourly?: number[] | null;
  humidity_hourly?: number[] | null;
  temp_now_normal?: number | null;
  temp_diff?: number | null;
  feels_like_now_normal?: number | null;
  wind_now_normal?: number | null;
  humidity_now_normal?: number | null;
};

export type PrecipitationToDate = {
  station?: StationInfo | null;
  today_observed_mm?: number | null;
  today_normal_mm?: number | null;
  month_to_date_observed_mm?: number | null;
  month_to_date_normal_mm?: number | null;
  month_normal_mm?: number | null;
  observed_through?: string | null;
};

export type DailyClimateNormalsResponse = {
  station: StationInfo;
  hourly_station?: StationInfo | null;
  period: string;
  today: DailyNormalToday;
  precipitation?: PrecipitationToDate | null;
  daily: DailyNormal[];
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

type SignedFetchInput = {
  config: WebConfig;
  path: string;
  params: Record<string, string>;
  timestamp: string;
  fetchImpl: typeof fetch;
};

// Performs an HMAC-signed GET against the Go API and returns the raw Response.
// Used for non-JSON payloads (e.g. map overlay PNGs). JSON callers should use
// signedGet, which parses and checks the status on top of this.
export async function signedFetch({
  config,
  path,
  params,
  timestamp,
  fetchImpl,
}: SignedFetchInput): Promise<Response> {
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

  // Retry once on a connection-level failure. Node's fetch pools keep-alive
  // sockets, so an occasional reused-but-closed socket throws ("other side
  // closed"); these GETs are idempotent, so a fresh-connection retry is safe.
  // Don't retry deliberate aborts, and rethrow the *original* error if the retry
  // also fails — it's the more diagnostic one.
  try {
    return await fetchImpl(url, { method: "GET", headers });
  } catch (err) {
    if (err instanceof Error && err.name === "AbortError") throw err;
    await new Promise((resolve) => setTimeout(resolve, 50));
    try {
      return await fetchImpl(url, { method: "GET", headers });
    } catch {
      throw err;
    }
  }
}

// Builds a JSON error Response for the map proxy routes. Shared so every failure
// path (bad request, upstream error, and an upstream connection failure) returns
// the same shape instead of letting a thrown fetch escape as an opaque 500.
export function jsonError(message: string, status: number): Response {
  return new Response(JSON.stringify({ error: message }), {
    status,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "no-store",
    },
  });
}

export class WeatherApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(`Weather API failed with ${status}: ${message}`);
    this.name = "WeatherApiError";
    this.status = status;
  }
}

async function signedGet<T>(input: SignedFetchInput): Promise<T> {
  const response = await signedFetch(input);
  if (!response.ok) {
    throw new WeatherApiError(response.status, await errorMessage(response));
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

export async function fetchDailyClimateNormals({
  city,
  config,
  timestamp = String(Math.floor(Date.now() / 1000)),
  fetchImpl = fetch,
}: FetchWeatherInput): Promise<DailyClimateNormalsResponse> {
  return signedGet<DailyClimateNormalsResponse>({
    config,
    path: "/v1/climate-normals/daily",
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
export function pageCacheControl(policy: {
  ttlSeconds: number;
  staleSeconds: number;
}): string {
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
