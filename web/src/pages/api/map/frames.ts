import type { APIRoute } from "astro";
import { proxyMapRequest } from "../../../lib/mapProxy";

export const prerender = false;

// Signed proxy for the map frame manifest: the per-layer list of scrubbable
// frame instants. The client snaps its `time`/`at` params to these so overlay
// URLs stay stable across scrubs and can be served from HTTP caches.
export const GET: APIRoute = () =>
  proxyMapRequest({
    label: "map frames",
    path: "/v1/map/frames",
    params: {},
    cache: { ttlSeconds: 60, staleSeconds: 60 },
    contentType: "application/json",
  });
