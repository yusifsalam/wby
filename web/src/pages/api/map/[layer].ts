import type { APIRoute } from "astro";
import { readWebConfig } from "../../../lib/config";
import {
  isMapLayer,
  jsonError,
  pageCacheControl,
  signedFetch,
} from "../../../lib/weatherApi";

export const prerender = false;

// Query params forwarded verbatim to the Go overlay endpoints. The backend
// validates/clamps them, so the proxy only whitelists which keys pass through.
const FORWARDED_PARAMS = ["bbox", "width", "height", "time"] as const;

// Response headers worth surfacing to the browser (data timestamp + the temp
// min/max the client legend uses).
const FORWARDED_HEADERS = [
  "X-Data-Time",
  "X-Temp-Min",
  "X-Temp-Max",
  "X-Layer",
];

export const GET: APIRoute = async ({ params, url }) => {
  const layer = params.layer ?? "";
  if (!isMapLayer(layer)) {
    return jsonError(`unknown map layer: ${layer}`, 400);
  }

  const bbox = url.searchParams.get("bbox");
  if (!bbox) {
    return jsonError("bbox is required", 400);
  }

  const forwarded: Record<string, string> = {};
  for (const key of FORWARDED_PARAMS) {
    const value = url.searchParams.get(key);
    if (value !== null) {
      forwarded[key] = value;
    }
  }

  let config: ReturnType<typeof readWebConfig>;
  try {
    config = readWebConfig(process.env);
  } catch (error) {
    return jsonError(
      error instanceof Error ? error.message : "config error",
      500,
    );
  }

  let upstream: Response;
  try {
    upstream = await signedFetch({
      config,
      path: `/v1/map/${layer}`,
      params: forwarded,
      timestamp: String(Math.floor(Date.now() / 1000)),
      fetchImpl: fetch,
    });
  } catch (error) {
    console.error("map overlay upstream fetch failed", error);
    return jsonError("map overlay upstream unavailable", 502);
  }

  if (!upstream.ok) {
    return jsonError(
      `map overlay failed with ${upstream.status}`,
      upstream.status,
    );
  }

  const headers = new Headers({
    "Content-Type": upstream.headers.get("Content-Type") ?? "image/png",
    "Cache-Control": pageCacheControl({ ttlSeconds: 120, staleSeconds: 120 }),
  });
  for (const name of FORWARDED_HEADERS) {
    const value = upstream.headers.get(name);
    if (value !== null) {
      headers.set(name, value);
    }
  }

  return new Response(upstream.body, { status: 200, headers });
};
