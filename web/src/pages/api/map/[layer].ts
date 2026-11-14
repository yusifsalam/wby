import type { APIRoute } from "astro";
import { isOverlayPngLayer } from "../../../lib/mapLayers";
import { pickParams, proxyMapRequest } from "../../../lib/mapProxy";
import { jsonError } from "../../../lib/weatherApi";

export const prerender = false;

// Signed proxy for the server-rendered overlay PNGs. The backend validates
// and clamps the params, so the proxy only whitelists which keys pass through.
export const GET: APIRoute = async ({ params, url }) => {
  const layer = params.layer ?? "";
  if (!isOverlayPngLayer(layer)) {
    return jsonError(`unknown map layer: ${layer}`, 400);
  }
  if (!url.searchParams.get("bbox")) {
    return jsonError("bbox is required", 400);
  }

  return proxyMapRequest({
    label: "map overlay",
    path: `/v1/map/${layer}`,
    params: pickParams(url, ["bbox", "width", "height", "time"]),
    cache: { ttlSeconds: 120, staleSeconds: 120 },
    contentType: "image/png",
    forwardHeaders: ["X-Data-Time", "X-Temp-Min", "X-Temp-Max", "X-Layer"],
  });
};
