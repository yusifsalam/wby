export type Pt = readonly [number, number];
export type Rnd = () => number;

// Mulberry32: a small seeded generator, so every build draws the same lines.
export function rng(seed: number): Rnd {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

export function between(rnd: Rnd, min: number, max: number): number {
  return min + (max - min) * rnd();
}

export function n(value: number): string {
  const rounded = Math.round(value * 10) / 10;
  return Object.is(rounded, -0) ? "0" : String(rounded);
}

export function pt(p: Pt): string {
  return `${n(p[0])} ${n(p[1])}`;
}

function dist(a: Pt, b: Pt): number {
  return Math.hypot(b[0] - a[0], b[1] - a[1]);
}

function lerp(a: Pt, b: Pt, t: number): [number, number] {
  return [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t];
}

// A smooth Catmull-Rom curve through the points, as cubic Béziers.
export function curve(points: Pt[], closed = false): string {
  if (points.length < 2) return "";
  const p = closed
    ? [points.at(-1) as Pt, ...points, points[0], points[1]]
    : [points[0], ...points, points.at(-1) as Pt];
  let d = `M${pt(p[1])}`;
  for (let i = 1; i < p.length - 2; i++) {
    const [p0, p1, p2, p3] = [p[i - 1], p[i], p[i + 1], p[i + 2]];
    const c1: Pt = [p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6];
    const c2: Pt = [p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6];
    d += `C${pt(c1)} ${pt(c2)} ${pt(p2)}`;
  }
  return closed ? `${d}Z` : d;
}

export function polygon(points: Pt[]): string {
  return `M${points.map(pt).join("L")}Z`;
}

type LineOptions = { roughness?: number; bowing?: number; double?: boolean };

// A pencil line: one or two slightly bowed passes with wandering ends.
export function line(
  a: Pt,
  b: Pt,
  rnd: Rnd,
  options: LineOptions = {},
): string {
  const { roughness = 1, bowing = 1, double = true } = options;
  const len = Math.max(dist(a, b), 0.001);
  const amp = roughness * Math.min(2.4, 0.5 + len * 0.012);
  const nx = -(b[1] - a[1]) / len;
  const ny = (b[0] - a[0]) / len;
  const pass = () => {
    const j = () => (rnd() - 0.5) * amp;
    const bow = bowing * (rnd() - 0.5) * Math.min(len * 0.03, 6);
    const p0: Pt = [a[0] + j(), a[1] + j()];
    const p3: Pt = [b[0] + j(), b[1] + j()];
    const m1 = lerp(a, b, between(rnd, 0.2, 0.4));
    const m2 = lerp(a, b, between(rnd, 0.6, 0.8));
    const c1: Pt = [m1[0] + nx * (bow + j()), m1[1] + ny * (bow + j())];
    const c2: Pt = [m2[0] + nx * (bow + j()), m2[1] + ny * (bow + j())];
    return `M${pt(p0)}C${pt(c1)} ${pt(c2)} ${pt(p3)}`;
  };
  return double ? pass() + pass() : pass();
}

// A sketched outline: straight strokes between the points that overshoot the
// corners a little, as a hand drawing does.
export function outline(
  points: Pt[],
  rnd: Rnd,
  options: LineOptions & { closed?: boolean } = {},
): string {
  const { closed = true, ...rest } = options;
  let d = "";
  const last = closed ? points.length : points.length - 1;
  for (let i = 0; i < last; i++) {
    const a = points[i];
    const b = points[(i + 1) % points.length];
    const len = Math.max(dist(a, b), 0.001);
    const over = between(rnd, 0.5, 2.5) / len;
    d += line(lerp(a, b, -over), lerp(a, b, 1 + over), rnd, rest);
  }
  return d;
}

// A loose hand-drawn ellipse that overlaps where it closes.
export function ellipse(
  cx: number,
  cy: number,
  rx: number,
  ry: number,
  rnd: Rnd,
  options: { roughness?: number; double?: boolean } = {},
): string {
  const { roughness = 1, double = true } = options;
  const pass = () => {
    const count = 14;
    const start = rnd() * Math.PI * 2;
    const sweep = Math.PI * 2 + between(rnd, 0.25, 0.5);
    const pts: Pt[] = [];
    for (let i = 0; i <= count; i++) {
      const a = start + (sweep * i) / count;
      const k = 1 + (rnd() - 0.5) * 0.07 * roughness;
      pts.push([cx + rx * k * Math.cos(a), cy + ry * k * Math.sin(a)]);
    }
    return curve(pts);
  };
  return double ? pass() + pass() : pass();
}

// Coloured-pencil hatching clipped to a polygon.
export function hatch(
  poly: Pt[],
  rnd: Rnd,
  options: { angle?: number; gap?: number; roughness?: number } = {},
): string {
  const { angle = -40, gap = 6, roughness = 0.5 } = options;
  const rad = (angle * Math.PI) / 180;
  const cos = Math.cos(rad);
  const sin = Math.sin(rad);
  const rot = (p: Pt): Pt => [
    p[0] * cos + p[1] * sin,
    -p[0] * sin + p[1] * cos,
  ];
  const unrot = (p: Pt): Pt => [
    p[0] * cos - p[1] * sin,
    p[0] * sin + p[1] * cos,
  ];
  const r = poly.map(rot);
  const ys = r.map((p) => p[1]);
  let d = "";
  for (
    let y = Math.min(...ys) + gap * between(rnd, 0.3, 0.7);
    y < Math.max(...ys);
    y += gap * between(rnd, 0.85, 1.15)
  ) {
    const xs: number[] = [];
    for (let i = 0; i < r.length; i++) {
      const a = r[i];
      const b = r[(i + 1) % r.length];
      if ((a[1] <= y && b[1] > y) || (b[1] <= y && a[1] > y)) {
        xs.push(a[0] + ((y - a[1]) / (b[1] - a[1])) * (b[0] - a[0]));
      }
    }
    xs.sort((p, q) => p - q);
    for (let i = 0; i + 1 < xs.length; i += 2) {
      const inset = Math.min((xs[i + 1] - xs[i]) * 0.2, gap * 0.6);
      const x0 = xs[i] + between(rnd, 0, inset);
      const x1 = xs[i + 1] - between(rnd, 0, inset);
      if (x1 - x0 < 1.5) continue;
      d += line(unrot([x0, y]), unrot([x1, y]), rnd, {
        roughness,
        bowing: 0.4,
        double: false,
      });
    }
  }
  return d;
}

type BrushOptions = { taper?: number; wobble?: number; samples?: number };

// An ink brush stroke along a smooth path through the points: a filled shape
// that swells and thins along its length and tapers at both ends.
export function brush(
  points: Pt[],
  width: number,
  rnd: Rnd,
  options: BrushOptions = {},
): string {
  const { taper = 0.18, wobble = 0.3 } = options;
  const dense = sample(points, options.samples);
  const phases = [rnd() * 6.28, rnd() * 6.28, rnd() * 6.28];
  const left: Pt[] = [];
  const right: Pt[] = [];
  dense.forEach((p, i) => {
    const t = i / (dense.length - 1);
    const prev = dense[Math.max(0, i - 1)];
    const next = dense[Math.min(dense.length - 1, i + 1)];
    const len = Math.max(dist(prev, next), 0.001);
    const nx = -(next[1] - prev[1]) / len;
    const ny = (next[0] - prev[0]) / len;
    const ends = Math.min(1, t / taper, (1 - t) / taper);
    const swell =
      1 +
      wobble *
        (0.6 * Math.sin(phases[0] + t * 7) +
          0.4 * Math.sin(phases[1] + t * 17));
    const w =
      (width / 2) * Math.max(0.12, Math.sqrt(Math.max(ends, 0))) * swell;
    left.push([p[0] + nx * w, p[1] + ny * w]);
    right.push([p[0] - nx * w, p[1] - ny * w]);
  });
  return `M${[...left, ...right.reverse()].map(pt).join("L")}Z`;
}

// Points along the Catmull-Rom curve through `points`, roughly evenly spaced.
export function sample(points: Pt[], count?: number): Pt[] {
  if (points.length < 2) return [...points];
  const p = [points[0], ...points, points.at(-1) as Pt];
  const total = points
    .slice(1)
    .reduce((sum, q, i) => sum + dist(points[i], q), 0);
  const steps = count ?? Math.max(8, Math.round(total / 3));
  const out: Pt[] = [];
  for (let s = 0; s <= steps; s++) {
    const u = (s / steps) * (points.length - 1);
    const i = Math.min(Math.floor(u), points.length - 2) + 1;
    const t = u - (i - 1);
    const [p0, p1, p2, p3] = [p[i - 1], p[i], p[i + 1], p[i + 2]];
    const t2 = t * t;
    const t3 = t2 * t;
    const coord = (k: 0 | 1) =>
      0.5 *
      (2 * p1[k] +
        (-p0[k] + p2[k]) * t +
        (2 * p0[k] - 5 * p1[k] + 4 * p2[k] - p3[k]) * t2 +
        (-p0[k] + 3 * p1[k] - 3 * p2[k] + p3[k]) * t3);
    out.push([coord(0), coord(1)]);
  }
  return out;
}

// Points around an ellipse, for paper shapes.
export function ring(
  cx: number,
  cy: number,
  rx: number,
  ry: number,
  count = 36,
  start = 0,
): Pt[] {
  return Array.from({ length: count }, (_, i) => {
    const a = start + (i / count) * Math.PI * 2;
    return [cx + rx * Math.cos(a), cy + ry * Math.sin(a)] as Pt;
  });
}

// A puffy cloud silhouette: the upper envelope of overlapping circles sitting
// on a gently wavering base.
export function cloudShape(
  x: number,
  y: number,
  w: number,
  h: number,
  rnd: Rnd,
  bumps = 5,
): Pt[] {
  const base = y + h;
  const end = h * 0.3;
  const circles: [number, number, number][] = [
    [x + end, base - end, end],
    [x + w - end, base - end, end],
  ];
  for (let i = 0; i < bumps; i++) {
    const t = bumps === 1 ? 0.5 : i / (bumps - 1);
    const peak = Math.sin(Math.PI * (0.15 + 0.7 * t));
    const r = h * (0.32 + 0.3 * peak) * between(rnd, 0.86, 1.06);
    const cx = x + w * (0.16 + 0.68 * t) + between(rnd, -w * 0.03, w * 0.03);
    circles.push([cx, base - r * 0.62, r]);
  }
  const top: Pt[] = [];
  for (let px = x + 2; px <= x + w - 1.99; px += 2.5) {
    let best = Number.POSITIVE_INFINITY;
    for (const [cx, cy, r] of circles) {
      const dx = px - cx;
      if (Math.abs(dx) <= r)
        best = Math.min(best, cy - Math.sqrt(r * r - dx * dx));
    }
    if (Number.isFinite(best)) top.push([px, Math.min(best, base)]);
  }
  const cap = (cx: number, from: number, to: number): Pt[] =>
    Array.from({ length: 6 }, (_, i) => {
      const a = from + ((to - from) * i) / 5;
      return [cx + end * Math.cos(a), base - end + end * Math.sin(a)] as Pt;
    });
  const bottom: Pt[] = [];
  for (let i = 5; i > 0; i--)
    bottom.push([
      x + end + ((w - 2 * end) * i) / 6,
      base + between(rnd, -0.6, 1.2),
    ]);
  return [
    ...cap(x + end, Math.PI / 2, Math.PI * 0.97),
    ...top,
    ...cap(x + w - end, Math.PI * 0.03, Math.PI / 2),
    ...bottom,
  ];
}

// The upper edge of a cloudShape, for outlining just the puffy top.
export function cloudTop(points: Pt[], every = 4): Pt[] {
  const base = Math.max(...points.map((p) => p[1]));
  return points.filter((p, i) => p[1] < base - 3 && i % every === 0);
}
