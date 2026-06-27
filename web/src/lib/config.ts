import type { WebConfig } from "./weatherApi";

export type WebRuntimeConfig = WebConfig & {
  ttlSeconds: number;
  staleSeconds: number;
};

type Env = Record<string, string | undefined>;

export function readWebConfig(env: Env): WebRuntimeConfig {
  return {
    apiBaseUrl: env.WBY_API_BASE_URL?.trim() || "http://localhost:8080",
    clientId: required(env, "WBY_API_CLIENT_ID"),
    clientSecret: required(env, "WBY_API_CLIENT_SECRET"),
    ttlSeconds: positiveInt(env.WBY_CACHE_TTL_SECONDS, 900),
    staleSeconds: positiveInt(env.WBY_CACHE_STALE_SECONDS, 3600),
  };
}

function required(env: Env, key: string): string {
  const value = env[key]?.trim();
  if (!value) {
    throw new Error(`${key} is required`);
  }
  return value;
}

function positiveInt(raw: string | undefined, fallback: number): number {
  if (!raw) {
    return fallback;
  }
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}
