import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { pickParams, proxyMapRequest } from "./mapProxy";
import { pageCacheControl } from "./weatherApi";

const request = {
  label: "map frames",
  path: "/v1/map/frames",
  params: {},
  cache: { ttlSeconds: 60, staleSeconds: 60 },
  contentType: "application/json",
};

describe("proxyMapRequest", () => {
  beforeEach(() => {
    vi.stubEnv("WBY_API_CLIENT_ID", "web");
    vi.stubEnv("WBY_API_CLIENT_SECRET", "test-secret");
    vi.stubEnv("WBY_API_BASE_URL", "http://server:8080");
    vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
  });

  it("relays the body with its own cache policy and the listed headers", async () => {
    vi.stubGlobal(
      "fetch",
      async () =>
        new Response("png-bytes", {
          headers: {
            "Content-Type": "image/png",
            "X-Data-Time": "2026-11-14T14:00:00Z",
            "X-Internal": "hidden",
          },
        }),
    );

    const res = await proxyMapRequest({
      ...request,
      contentType: undefined,
      forwardHeaders: ["X-Data-Time"],
    });

    expect(res.status).toBe(200);
    expect(await res.text()).toBe("png-bytes");
    expect(res.headers.get("Content-Type")).toBe("image/png");
    expect(res.headers.get("X-Data-Time")).toBe("2026-11-14T14:00:00Z");
    expect(res.headers.get("X-Internal")).toBeNull();
    expect(res.headers.get("Cache-Control")).toBe(
      pageCacheControl(request.cache),
    );
  });

  it("passes on an upstream status the browser can act on", async () => {
    vi.stubGlobal("fetch", async () => new Response("{}", { status: 404 }));
    const res = await proxyMapRequest(request);
    expect(res.status).toBe(404);
    expect(await res.json()).toEqual({ error: "map frames failed with 404" });
    expect(res.headers.get("Cache-Control")).toBe("no-store");
  });

  it("hides a rejection of its own credentials behind a 502", async () => {
    vi.stubGlobal("fetch", async () => new Response("{}", { status: 401 }));
    const res = await proxyMapRequest(request);
    expect(res.status).toBe(502);
    expect(await res.json()).toEqual({
      error: "map frames upstream unavailable",
    });
  });

  it("does not put configuration details in the response", async () => {
    vi.stubEnv("WBY_API_CLIENT_SECRET", "");
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);

    const res = await proxyMapRequest(request);

    expect(res.status).toBe(502);
    expect(await res.text()).not.toContain("WBY_API_CLIENT_SECRET");
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(console.error).toHaveBeenCalled();
  });

  it("answers 502 when the upstream runs out of time, without a retry", async () => {
    let calls = 0;
    vi.stubGlobal("fetch", async () => {
      calls += 1;
      throw new DOMException("timed out", "TimeoutError");
    });

    const res = await proxyMapRequest(request);

    expect(res.status).toBe(502);
    expect(calls).toBe(1);
  });
});

describe("pickParams", () => {
  it("keeps only the listed parameters that have a value", () => {
    const url = new URL(
      "http://localhost/api/map/x?bbox=19,59,32,71&time=&secret=1&width=800",
    );
    expect(pickParams(url, ["bbox", "width", "height", "time"])).toEqual({
      bbox: "19,59,32,71",
      width: "800",
    });
  });
});
