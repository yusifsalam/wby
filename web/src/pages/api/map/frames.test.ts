import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GET } from "./frames";

type Ctx = Parameters<typeof GET>[0];

describe("map frames proxy", () => {
  beforeEach(() => {
    vi.stubEnv("WBY_API_CLIENT_ID", "web");
    vi.stubEnv("WBY_API_CLIENT_SECRET", "test-secret");
    vi.stubEnv("WBY_API_BASE_URL", "http://server:8080");
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
  });

  it("signs and forwards the frames request", async () => {
    const manifest = {
      generated_at: "2026-07-02T05:37:12Z",
      temperature: {
        times: ["2026-07-02T05:00:00Z"],
        now_index: 0,
        step_seconds: 3600,
      },
      precipitation: {
        times: ["2026-07-02T05:35:00Z"],
        now_index: 0,
        step_seconds: 300,
      },
    };
    const calls: Array<{ url: string; headers: Headers }> = [];
    vi.stubGlobal("fetch", async (url: string | URL, init?: RequestInit) => {
      calls.push({ url: String(url), headers: new Headers(init?.headers) });
      return Response.json(manifest);
    });

    const res = await GET({} as Ctx);

    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("application/json");
    expect(await res.json()).toEqual(manifest);

    expect(calls).toHaveLength(1);
    expect(calls[0].url).toBe("http://server:8080/v1/map/frames");
    expect(calls[0].headers.get("X-Client-ID")).toBe("web");
    expect(calls[0].headers.get("X-Signature")).toMatch(/^[a-f0-9]{64}$/);
  });

  it("propagates an upstream failure as JSON", async () => {
    vi.stubGlobal("fetch", async () => new Response("nope", { status: 502 }));

    const res = await GET({} as Ctx);

    expect(res.status).toBe(502);
    expect(await res.json()).toEqual({ error: "map frames failed with 502" });
  });
});
