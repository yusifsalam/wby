import type { APIRoute } from "astro";
import { pickParams, proxyMapRequest } from "../../../lib/mapProxy";

export const prerender = false;

// Signed proxy for the temperature samples/grid endpoint. For a future `at` the
// response carries a dense GRIB grid the browser rasterizes client-side (matching
// the iOS render); for now/past it carries station point samples. The upstream
// takes the instant as `at` (RFC3339).
export const GET: APIRoute = ({ url }) =>
  proxyMapRequest({
    label: "temperature samples",
    path: "/v1/map/temperature/samples",
    params: pickParams(url, ["at"]),
    cache: { ttlSeconds: 120, staleSeconds: 120 },
    contentType: "application/json",
  });
