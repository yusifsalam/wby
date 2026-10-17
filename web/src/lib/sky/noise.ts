/**
 * CPU mirror of the cloud volume in sky.frag, used to dim the lens flare when
 * clouds cover the sun. Port of ios/wby/wby/Background/SkyNoise.swift.
 */

const PITCH = 0.5;
const TAN_HALF_FOV = 0.8;
const LAYER_BOTTOM = 1;
const SUN_U = 0.2;
const SUN_V = 0.04;

type Vec3 = [number, number, number];

export function hash3(x: number, y: number, z: number): number {
  let n =
    (Math.imul(x, 1597334673) ^
      Math.imul(y, 3812015801) ^
      Math.imul(z, 2798796415)) >>>
    0;
  n = Math.imul(n ^ (n >>> 15), 2246822519) >>> 0;
  n = Math.imul(n ^ (n >>> 13), 3266489917) >>> 0;
  return ((n ^ (n >>> 16)) >>> 0) / 4294967295;
}

const mix = (a: number, b: number, t: number) => a + (b - a) * t;

function smoothstep(edge0: number, edge1: number, x: number): number {
  const t = Math.min(Math.max((x - edge0) / (edge1 - edge0), 0), 1);
  return t * t * (3 - 2 * t);
}

function noise3(px: number, py: number, pz: number): number {
  const ix = Math.floor(px);
  const iy = Math.floor(py);
  const iz = Math.floor(pz);
  const fx = px - ix;
  const fy = py - iy;
  const fz = pz - iz;
  const ux = fx * fx * (3 - 2 * fx);
  const uy = fy * fy * (3 - 2 * fy);
  const uz = fz * fz * (3 - 2 * fz);
  const x00 = mix(hash3(ix, iy, iz), hash3(ix + 1, iy, iz), ux);
  const x10 = mix(hash3(ix, iy + 1, iz), hash3(ix + 1, iy + 1, iz), ux);
  const x01 = mix(hash3(ix, iy, iz + 1), hash3(ix + 1, iy, iz + 1), ux);
  const x11 = mix(hash3(ix, iy + 1, iz + 1), hash3(ix + 1, iy + 1, iz + 1), ux);
  return mix(mix(x00, x10, uy), mix(x01, x11, uy), uz);
}

function rotate3([x, y, z]: Vec3, scale: number): Vec3 {
  return [
    (-0.8 * y - 0.6 * z) * scale,
    (0.8 * x + 0.36 * y - 0.48 * z) * scale,
    (0.6 * x - 0.48 * y + 0.64 * z) * scale,
  ];
}

function viewRay(u: number, v: number, aspect: number): Vec3 {
  const nx = (u - 0.5) * 2 * aspect * TAN_HALF_FOV;
  const ny = (0.5 - v) * 2 * TAN_HALF_FOV;
  const s = Math.sin(PITCH);
  const c = Math.cos(PITCH);
  const x = nx;
  const y = s + c * ny;
  const z = c - s * ny;
  const length = Math.hypot(x, y, z);
  return [x / length, y / length, z / length];
}

function density(
  p: Vec3,
  coverage: number,
  deck: number,
  top: number,
  offset: Vec3,
): number {
  const h = (p[1] - LAYER_BOTTOM) / (top - LAYER_BOTTOM);
  const cumulusProfile = smoothstep(0, 0.12, h) * (1 - smoothstep(0.25, 1, h));
  const deckProfile = smoothstep(0, 0.1, h) * (1 - smoothstep(0.55, 1, h));
  const profile = mix(cumulusProfile, deckProfile, deck);
  if (profile <= 0) return 0;

  const scale = mix(3, 3.8, deck);
  let q: Vec3 = [
    p[0] * scale + offset[0],
    p[1] * scale + offset[1],
    p[2] * scale + offset[2],
  ];
  let f = 0.5 * noise3(...q);
  q = rotate3(q, 2.02);
  f += 0.25 * noise3(...q);
  q = rotate3(q, 2.03);
  f += 0.125 * noise3(...q);
  f /= 0.875;

  const threshold = mix(0.78, 0.3, coverage);
  return Math.min(
    Math.max(((f * profile - threshold) / (1 - threshold)) * 3.2, 0),
    1,
  );
}

function cloudAlpha(
  rd: Vec3,
  time: number,
  drift: readonly [number, number],
  coverage: number,
  deck: number,
): number {
  if (rd[1] < 0.03) return 0;
  const top = LAYER_BOTTOM + mix(0.45, 0.65, deck);
  const t0 = LAYER_BOTTOM / rd[1];
  if (t0 > 9) return 0;
  const t1 = Math.min(top / rd[1], 9);
  const steps = 12;
  const dt = (t1 - t0) / steps;
  const offset: Vec3 = [-drift[0], time * 0.003, -drift[1]];
  let t = t0 + dt * 0.5;
  let alpha = 0;
  for (let i = 0; i < steps; i++) {
    const den = density(
      [rd[0] * t, rd[1] * t, rd[2] * t],
      coverage,
      deck,
      top,
      offset,
    );
    if (den > 0.003) {
      alpha += (1 - Math.exp(-den * dt * 14)) * (1 - alpha);
      if (alpha > 0.97) break;
    }
    t += dt;
  }
  return alpha;
}

export function sunOcclusion(
  width: number,
  height: number,
  time: number,
  drift: readonly [number, number],
  coverage: number,
  deck: number,
): number {
  if ((coverage <= 0.001 && deck <= 0.001) || width <= 0 || height <= 0)
    return 0;
  const aspect = width / height;
  const r = Math.min(width, height) * 0.12;
  const offsets: [number, number][] = [
    [0, 0],
    [r, 0],
    [-r, 0],
    [0, r],
    [0, -r * 0.3],
  ];
  let total = 0;
  for (const [dx, dy] of offsets) {
    const rd = viewRay(SUN_U + dx / width, SUN_V + dy / height, aspect);
    total += cloudAlpha(rd, time, drift, coverage, deck);
  }
  return total / offsets.length;
}
