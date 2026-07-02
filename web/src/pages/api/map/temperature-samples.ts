import type { APIRoute } from "astro";
import { readWebConfig } from "../../../lib/config";
import {
  jsonError,
  pageCacheControl,
  signedFetch,
} from "../../../lib/weatherApi";

export const prerender = false;

// Signed proxy for the temperature samples/grid endpoint. For a future `at` the
// response carries a dense GRIB grid the browser rasterizes client-side (matching
// the iOS render); for now/past it carries station point samples. The upstream
// uses `at` (RFC3339), distinct from the PNG overlay's `time`.
export const GET: APIRoute = async ({ url }) => {
  const params: Record<string, string> = {};
  const at = url.searchParams.get("at");
  if (at) params.at = at;

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
      path: "/v1/map/temperature/samples",
      params,
      timestamp: String(Math.floor(Date.now() / 1000)),
      fetchImpl: fetch,
    });
  } catch (error) {
    console.error("temperature samples upstream fetch failed", error);
    return jsonError("temperature samples upstream unavailable", 502);
  }

  if (!upstream.ok) {
    return jsonError(
      `temperature samples failed with ${upstream.status}`,
      upstream.status,
    );
  }

  return new Response(upstream.body, {
    status: 200,
    headers: new Headers({
      "Content-Type": "application/json",
      "Cache-Control": pageCacheControl({ ttlSeconds: 120, staleSeconds: 120 }),
    }),
  });
};
