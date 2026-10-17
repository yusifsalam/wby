import { describe, expect, it } from "vitest";
import { hash3, sunOcclusion } from "./noise";

function referenceHash3(x: number, y: number, z: number): number {
  const u32 = (v: bigint) => BigInt.asUintN(32, v);
  const bits = (v: number) => u32(BigInt(v));
  let n =
    u32(bits(x) * 1597334673n) ^
    u32(bits(y) * 3812015801n) ^
    u32(bits(z) * 2798796415n);
  n = u32((n ^ (n >> 15n)) * 2246822519n);
  n = u32((n ^ (n >> 13n)) * 3266489917n);
  return Number(n ^ (n >> 16n)) / 4294967295;
}

describe("sky noise", () => {
  it("matches 32-bit unsigned hash arithmetic", () => {
    for (const [x, y, z] of [
      [0, 0, 0],
      [1, 2, 3],
      [-7, 12, -1],
      [123456, -98765, 4321],
    ]) {
      expect(hash3(x, y, z)).toBe(referenceHash3(x, y, z));
    }
  });

  it("measures sun occlusion between 0 and 1", () => {
    expect(sunOcclusion(390, 844, 0, [0, 0], 0, 0)).toBe(0);
    const overcast = sunOcclusion(390, 844, 0, [0, 0], 1, 1);
    expect(overcast).toBeGreaterThan(0.5);
    expect(overcast).toBeLessThanOrEqual(1);
  });
});
