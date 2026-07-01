import { describe, expect, it } from "vitest";
import { createSignedHeaders, signaturePayload } from "./apiSignature";

describe("signaturePayload", () => {
  it("matches the Go server signing payload format", () => {
    expect(
      signaturePayload({
        method: "GET",
        path: "/v1/weather",
        rawQuery: "lat=60.1699&lon=24.9384",
        timestamp: "1769500800",
      }),
    ).toBe("GET\n/v1/weather\nlat=60.1699&lon=24.9384\n1769500800");
  });
});

describe("createSignedHeaders", () => {
  it("creates Go-compatible request signature headers", () => {
    expect(
      createSignedHeaders({
        clientId: "web",
        clientSecret: "test-secret",
        method: "GET",
        path: "/v1/weather",
        rawQuery: "lat=60.1699&lon=24.9384",
        timestamp: "1769500800",
      }),
    ).toEqual({
      "X-Client-ID": "web",
      "X-Timestamp": "1769500800",
      "X-Signature":
        "2dd7fac401190a36426d6962abbc92d59ba892b02429c044497d3d3e552670b0",
    });
  });
});
