import type { WebConfig } from "./weatherApi";

export type WebRuntimeConfig = WebConfig & {
  ttlSeconds: number;
  staleSeconds: number;
};

type Env = Record<string, string | undefined>;

// Reads the runtime configuration, throwing one error that lists every
// invalid or missing variable.
export function readWebConfig(env: Env): WebRuntimeConfig {
  const problems: string[] = [];

  const required = (key: string): string => {
    const value = env[key]?.trim();
    if (!value) {
      problems.push(`${key} is required`);
    }
    return value ?? "";
  };

  const seconds = (key: string, fallback: number): number => {
    const raw = env[key]?.trim();
    if (!raw) {
      return fallback;
    }
    if (!/^[1-9]\d*$/.test(raw)) {
      problems.push(`${key} must be a whole number of seconds above zero`);
      return fallback;
    }
    return Number(raw);
  };

  const apiBaseUrl = env.WBY_API_BASE_URL?.trim() || "http://localhost:8080";
  if (!/^https?:\/\//.test(apiBaseUrl) || !URL.canParse(apiBaseUrl)) {
    problems.push("WBY_API_BASE_URL must be an http or https URL");
  }

  const config = {
    apiBaseUrl,
    clientId: required("WBY_API_CLIENT_ID"),
    clientSecret: required("WBY_API_CLIENT_SECRET"),
    ttlSeconds: seconds("WBY_CACHE_TTL_SECONDS", 900),
    staleSeconds: seconds("WBY_CACHE_STALE_SECONDS", 3600),
  };
  if (problems.length > 0) {
    throw new Error(problems.join("; "));
  }
  return config;
}
