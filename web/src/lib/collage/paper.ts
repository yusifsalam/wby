import { between, n, type Pt, polygon, type Rnd, ring } from "./sketch";

function edgePoints(a: Pt, b: Pt, rnd: Rnd, amp: number, step: number): Pt[] {
  const len = Math.hypot(b[0] - a[0], b[1] - a[1]);
  const count = Math.max(1, Math.round(len / step));
  const nx = -(b[1] - a[1]) / len;
  const ny = (b[0] - a[0]) / len;
  const k1 = between(rnd, 2, 5);
  const k2 = between(rnd, 9, 16);
  const p1 = rnd() * 6.28;
  const p2 = rnd() * 6.28;
  const pts: Pt[] = [];
  for (let i = 0; i < count; i++) {
    const t = i / count;
    const wave = 0.55 * Math.sin(p1 + t * k1) + 0.3 * Math.sin(p2 + t * k2);
    const ease = Math.min(1, t * 6, (1 - t) * 6);
    const off = amp * (wave * ease + (rnd() - 0.5) * 0.9);
    pts.push([
      a[0] + (b[0] - a[0]) * t + nx * off,
      a[1] + (b[1] - a[1]) * t + ny * off,
    ]);
  }
  return pts;
}

// A torn-paper outline around a polygon: fibrous edges where `torn` is true,
// slightly wavering scissor cuts elsewhere.
export function torn(
  points: Pt[],
  rnd: Rnd,
  options: { amp?: number; step?: number; torn?: boolean[] } = {},
): string {
  const { amp = 2.6, step = 4 } = options;
  const out: Pt[] = [];
  points.forEach((a, i) => {
    const b = points[(i + 1) % points.length];
    const rough = options.torn?.[i] ?? true;
    out.push(
      ...edgePoints(
        a,
        b,
        rnd,
        rough ? amp : amp * 0.12,
        rough ? step : step * 6,
      ),
    );
  });
  return polygon(out);
}

// A scissor-cut outline: the corners land a little off and long edges bend
// where the scissors were repositioned.
export function cut(points: Pt[], rnd: Rnd, jitter = 1.2): string {
  const out: Pt[] = [];
  points.forEach((a, i) => {
    const b = points[(i + 1) % points.length];
    out.push([a[0] + (rnd() - 0.5) * jitter, a[1] + (rnd() - 0.5) * jitter]);
    const len = Math.hypot(b[0] - a[0], b[1] - a[1]);
    const cuts = Math.floor(len / 60);
    for (let c = 1; c <= cuts; c++) {
      const t = c / (cuts + 1) + (rnd() - 0.5) * 0.1;
      out.push([
        a[0] + (b[0] - a[0]) * t + (rnd() - 0.5) * jitter * 1.4,
        a[1] + (b[1] - a[1]) * t + (rnd() - 0.5) * jitter * 1.4,
      ]);
    }
  });
  return polygon(out);
}

export function cutRect(
  x: number,
  y: number,
  w: number,
  h: number,
  rnd: Rnd,
  jitter = 1.2,
): string {
  return cut(
    [
      [x, y],
      [x + w, y],
      [x + w, y + h],
      [x, y + h],
    ],
    rnd,
    jitter,
  );
}

export function tornRect(
  x: number,
  y: number,
  w: number,
  h: number,
  rnd: Rnd,
  options: { amp?: number; step?: number; torn?: boolean[] } = {},
): string {
  return torn(
    [
      [x, y],
      [x + w, y],
      [x + w, y + h],
      [x, y + h],
    ],
    rnd,
    options,
  );
}

// A circle cut freehand with scissors: a polygon with small facets.
export function cutCircle(
  cx: number,
  cy: number,
  r: number,
  rnd: Rnd,
  facets = 22,
): string {
  const pts = ring(cx, cy, r, r, facets, rnd() * 6.28).map(
    ([x, y]) =>
      [x + (rnd() - 0.5) * r * 0.05, y + (rnd() - 0.5) * r * 0.05] as Pt,
  );
  return polygon(pts);
}

// A strip of tape centred on (cx, cy): straight sides, ragged torn ends.
export function tape(
  cx: number,
  cy: number,
  w: number,
  h: number,
  angle: number,
  rnd: Rnd,
): string {
  const rad = (angle * Math.PI) / 180;
  const cos = Math.cos(rad);
  const sin = Math.sin(rad);
  const local: Pt[] = [];
  const teeth = Math.max(4, Math.round(h / 3));
  local.push([-w / 2, -h / 2], [w / 2, -h / 2]);
  for (let i = 1; i < teeth; i++)
    local.push([w / 2 + between(rnd, -1.6, 1.6), -h / 2 + (h * i) / teeth]);
  local.push([w / 2, h / 2], [-w / 2, h / 2]);
  for (let i = teeth - 1; i > 0; i--)
    local.push([-w / 2 + between(rnd, -1.6, 1.6), -h / 2 + (h * i) / teeth]);
  return polygon(
    local.map(
      ([x, y]) => [cx + x * cos - y * sin, cy + x * sin + y * cos] as Pt,
    ),
  );
}

export function translate(d: string, dx: number, dy: number): string {
  let i = 0;
  return d.replace(/-?\d+(\.\d+)?/g, (m) =>
    n(Number(m) + (i++ % 2 === 0 ? dx : dy)),
  );
}
