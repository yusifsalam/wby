import type { APIRoute } from "astro";
import { pickParams, proxyMapRequest } from "../../../lib/mapProxy";

export const prerender = false;

// Signed proxy for the Harmonie 12h precipitation forecast grid. The response
// carries a dense GRIB raster (mm/h) the browser rasterizes client-side with
// the shared precip ramp, matching the iOS render. The backend validates and
// clamps the params, so the proxy only whitelists which keys pass through.
export const GET: APIRoute = ({ url }) =>
  proxyMapRequest({
    label: "precipitation forecast",
    path: "/v1/map/precipitation/forecast",
    params: pickParams(url, ["bbox", "width", "height", "time"]),
    // Matches the upstream's own policy — the field only changes hourly.
    cache: { ttlSeconds: 300, staleSeconds: 900 },
    contentType: "application/json",
  });
