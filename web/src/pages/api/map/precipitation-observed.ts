import type { APIRoute } from "astro";
import { pickParams, proxyMapRequest } from "../../../lib/mapProxy";

export const prerender = false;

// Signed proxy for the keyless radar-composite observation grid (5-min
// frames, mm/h). Same dense-raster contract as the forecast grid proxy; the
// backend validates and clamps the params, so the proxy only whitelists which
// keys pass through.
export const GET: APIRoute = ({ url }) =>
  proxyMapRequest({
    label: "precipitation observed",
    path: "/v1/map/precipitation/observed",
    params: pickParams(url, ["bbox", "width", "height", "time"]),
    // Matches the upstream's own policy — a published frame never changes.
    cache: { ttlSeconds: 120, staleSeconds: 300 },
    contentType: "application/json",
  });
