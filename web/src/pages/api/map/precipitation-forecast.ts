import type { APIRoute } from "astro";
import { readWebConfig } from "../../../lib/config";
import {
  jsonError,
  pageCacheControl,
  signedFetch,
} from "../../../lib/weatherApi";

export const prerender = false;

// Signed proxy for the Harmonie 12h precipitation forecast grid. The response
// carries a dense GRIB raster (mm/h) the browser rasterizes client-side with
// the shared precip ramp, matching the iOS render. The backend validates and
// clamps the params, so the proxy only whitelists which keys pass through.
const FORWARDED_PARAMS = ["bbox", "width", "height", "time"] as const;

export const GET: APIRoute = async ({ url }) => {
  const params: Record<string, string> = {};
  for (const key of FORWARDED_PARAMS) {
    const value = url.searchParams.get(key);
    if (value !== null) {
      params[key] = value;
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
      path: "/v1/map/precipitation/forecast",
      params,
      timestamp: String(Math.floor(Date.now() / 1000)),
      fetchImpl: fetch,
    });
  } catch (error) {
    console.error("precipitation forecast upstream fetch failed", error);
    return jsonError("precipitation forecast upstream unavailable", 502);
  }

  if (!upstream.ok) {
    return jsonError(
      `precipitation forecast failed with ${upstream.status}`,
      upstream.status,
    );
  }

  return new Response(upstream.body, {
    status: 200,
    headers: new Headers({
      "Content-Type": "application/json",
      // Matches the upstream's own policy — the field only changes hourly.
      "Cache-Control": pageCacheControl({ ttlSeconds: 300, staleSeconds: 900 }),
    }),
  });
};
