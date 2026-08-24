import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GET } from "./[layer]";

type Ctx = Parameters<typeof GET>[0];

function context(layer: string, query: string): Ctx {
  return {
    params: { layer },
    url: new URL(`http://web.local/api/map/${layer}?${query}`),
  } as unknown as Ctx;
}

describe("map overlay proxy", () => {
  beforeEach(() => {
    vi.stubEnv("WBY_API_CLIENT_ID", "web");
    vi.stubEnv("WBY_API_CLIENT_SECRET", "test-secret");
    vi.stubEnv("WBY_API_BASE_URL", "http://server:8080");
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
  });

  it("signs and forwards a temperature overlay request", async () => {
    const png = new Uint8Array([137, 80, 78, 71]);
    const calls: Array<{ url: string; headers: Headers }> = [];
    vi.stubGlobal("fetch", async (url: string | URL, init?: RequestInit) => {
      calls.push({ url: String(url), headers: new Headers(init?.headers) });
      return new Response(png, {
        status: 200,
        headers: {
          "Content-Type": "image/png",
          "X-Data-Time": "2026-07-01T12:00:00Z",
        },
      });
    });

    const res = await GET(
      context("temperature", "bbox=19,59,32,71&width=800&height=600"),
    );

    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("image/png");
    expect(res.headers.get("X-Data-Time")).toBe("2026-07-01T12:00:00Z");

    expect(calls).toHaveLength(1);
    expect(calls[0].url).toBe(
      "http://server:8080/v1/map/temperature?bbox=19%2C59%2C32%2C71&width=800&height=600",
    );
    expect(calls[0].headers.get("X-Client-ID")).toBe("web");
    expect(calls[0].headers.get("X-Signature")).toMatch(/^[a-f0-9]{64}$/);
  });

  it("rejects an unknown layer", async () => {
    const res = await GET(context("humidity", "bbox=19,59,32,71"));
    expect(res.status).toBe(400);
  });

  it("rejects the grid-only 12h forecast layer", async () => {
    // precipitation12h has no PNG endpoint; it goes through the
    // precipitation-forecast proxy instead.
    const res = await GET(context("precipitation12h", "bbox=19,59,32,71"));
    expect(res.status).toBe(400);
  });

  it("requires a bbox", async () => {
    const res = await GET(context("precipitation", "width=800"));
    expect(res.status).toBe(400);
  });

  it("returns a JSON 502 when the upstream connection fails", async () => {
    // Both the initial attempt and signedFetch's one retry throw, mimicking the
    // Go server being unreachable. The route must not let the throw escape.
    vi.stubGlobal("fetch", async () => {
      throw new TypeError("fetch failed");
    });

    const res = await GET(context("temperature", "bbox=19,59,32,71"));

    expect(res.status).toBe(502);
    expect(res.headers.get("Content-Type")).toBe("application/json");
    expect(await res.json()).toEqual({
      error: "map overlay upstream unavailable",
    });
  });
});
