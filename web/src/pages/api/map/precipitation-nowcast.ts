import type { APIRoute } from "astro";
import { pickParams, proxyMapRequest } from "../../../lib/mapProxy";

export const prerender = false;

// Signed proxy for the radar-extrapolation nowcast grid (5-min future frames,
// mm/h). Same dense-raster contract as the observed grid proxy, but frames are
// re-predicted every run, so the cache window is short.
export const GET: APIRoute = ({ url }) =>
  proxyMapRequest({
    label: "precipitation nowcast",
    path: "/v1/map/precipitation/nowcast",
    params: pickParams(url, ["bbox", "width", "height", "time"]),
    // Matches the upstream's own policy — frames change on every run.
    cache: { ttlSeconds: 60, staleSeconds: 120 },
    contentType: "application/json",
  });
