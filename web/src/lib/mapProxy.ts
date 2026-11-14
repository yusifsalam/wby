import { readWebConfig } from "./config";
import {
  type CachePolicy,
  jsonError,
  pageCacheControl,
  signedFetch,
} from "./weatherApi";

type MapProxyRequest = {
  // Names the endpoint in logs and error bodies, e.g. "map frames".
  label: string;
  path: string;
  params: Record<string, string>;
  cache: CachePolicy;
  // Sent as the Content-Type; when absent the upstream's own is passed on.
  contentType?: string;
  forwardHeaders?: readonly string[];
};

// Signs a GET to the Go API and relays the answer to the browser, so the
// client secret stays on the server. Failures come back as JSON: the reason
// is logged here and the body names only the endpoint and status.
export async function proxyMapRequest({
  label,
  path,
  params,
  cache,
  contentType,
  forwardHeaders = [],
}: MapProxyRequest): Promise<Response> {
  let upstream: Response;
  try {
    upstream = await signedFetch({
      config: readWebConfig(process.env),
      path,
      params,
      timestamp: String(Math.floor(Date.now() / 1000)),
      fetchImpl: fetch,
    });
  } catch (error) {
    console.error(`${label} upstream fetch failed`, error);
    return jsonError(`${label} upstream unavailable`, 502);
  }

  if (upstream.status === 401 || upstream.status === 403) {
    console.error(`${label} rejected the proxy's own credentials`);
    return jsonError(`${label} upstream unavailable`, 502);
  }
  if (!upstream.ok) {
    return jsonError(
      `${label} failed with ${upstream.status}`,
      upstream.status,
    );
  }

  const headers = new Headers({
    "Content-Type":
      contentType ??
      upstream.headers.get("Content-Type") ??
      "application/octet-stream",
    "Cache-Control": pageCacheControl(cache),
  });
  for (const name of forwardHeaders) {
    const value = upstream.headers.get(name);
    if (value !== null) {
      headers.set(name, value);
    }
  }
  return new Response(upstream.body, { status: 200, headers });
}

// The query parameters named in `keys` that the request carries a value for.
export function pickParams(
  url: URL,
  keys: readonly string[],
): Record<string, string> {
  const picked: Record<string, string> = {};
  for (const key of keys) {
    const value = url.searchParams.get(key);
    if (value) {
      picked[key] = value;
    }
  }
  return picked;
}
