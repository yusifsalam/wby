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
  const url = new URL("/v1/weather", config.apiBaseUrl);
  url.searchParams.set("lat", formatCoordinate(city.latitude));
  url.searchParams.set("lon", formatCoordinate(city.longitude));

  const rawQuery = url.searchParams.toString();
  const headers = createSignedHeaders({
    clientId: config.clientId,
    clientSecret: config.clientSecret,
    method: "GET",
    path: url.pathname,
    rawQuery,
    timestamp,
  });

  const response = await fetchImpl(url, { method: "GET", headers });
  if (!response.ok) {
    throw new Error(`Weather API failed with ${response.status}: ${await errorMessage(response)}`);
  }

  return (await response.json()) as WeatherResponse;
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
