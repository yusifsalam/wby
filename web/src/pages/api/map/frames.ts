import type { APIRoute } from "astro";
import { readWebConfig } from "../../../lib/config";
import {
  jsonError,
  pageCacheControl,
  signedFetch,
} from "../../../lib/weatherApi";

export const prerender = false;

// Signed proxy for the map frame manifest: the per-layer list of scrubbable
// frame instants. The client snaps its `time`/`at` params to these so overlay
// URLs stay stable across scrubs and can be served from HTTP caches.
export const GET: APIRoute = async () => {
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
      path: "/v1/map/frames",
      params: {},
      timestamp: String(Math.floor(Date.now() / 1000)),
      fetchImpl: fetch,
    });
  } catch (error) {
    console.error("map frames upstream fetch failed", error);
    return jsonError("map frames upstream unavailable", 502);
  }

  if (!upstream.ok) {
    return jsonError(
      `map frames failed with ${upstream.status}`,
      upstream.status,
    );
  }

  return new Response(upstream.body, {
    status: 200,
    headers: new Headers({
      "Content-Type": "application/json",
      "Cache-Control": pageCacheControl({ ttlSeconds: 60, staleSeconds: 60 }),
    }),
  });
};
