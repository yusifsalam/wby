import { cut, cutRect } from "./paper";
import { boil, path, pencil } from "./scene";
import {
  between,
  curve,
  line,
  n,
  outline,
  type Pt,
  type Rnd,
  rng,
} from "./sketch";

export function grow(points: Pt[], by: number): Pt[] {
  const cx = points.reduce((s, p) => s + p[0], 0) / points.length;
  const cy = points.reduce((s, p) => s + p[1], 0) / points.length;
  return points.map(([x, y]) => {
    const len = Math.hypot(x - cx, y - cy) || 1;
    return [x + ((x - cx) / len) * by, y + ((y - cy) / len) * by] as Pt;
  });
}

export function band(points: Pt[], fromY: number): Pt[] {
  return points.map(([x, y]) => [x, Math.max(y, fromY)] as Pt);
}

export function arcPoints(
  cx: number,
  cy: number,
  r: number,
  from: number,
  to: number,
  count = 12,
): Pt[] {
  return Array.from({ length: count + 1 }, (_, i) => {
    const a = from + ((to - from) * i) / count;
    return [cx + r * Math.cos(a), cy + r * Math.sin(a)] as Pt;
  });
}

export function leafShape(
  cx: number,
  cy: number,
  size: number,
  angle: number,
  tone: string,
  rr: Rnd,
): string {
  const pts: Pt[] = [
    [0, -size],
    [size * 0.55, -size * 0.25],
    [size * 0.45, size * 0.45],
    [0, size],
    [-size * 0.45, size * 0.45],
    [-size * 0.55, -size * 0.25],
  ];
  const rad = (angle * Math.PI) / 180;
  const rot = pts.map(
    ([x, y]) =>
      [
        cx + x * Math.cos(rad) - y * Math.sin(rad),
        cy + x * Math.sin(rad) + y * Math.cos(rad),
      ] as Pt,
  );
  const tip = rot[0];
  const stem = rot[3];
  return (
    path(cut(rot, rr, size * 0.12), tone) +
    pencil(
      `M${n(stem[0])} ${n(stem[1])}L${n(tip[0])} ${n(tip[1])}`,
      "sun-deep",
      0.9,
      'opacity="0.8"',
    )
  );
}

// A Stevenson screen: the louvred white box on legs that shelters a weather
// station's thermometers.
export function stevensonScreen(
  x: number,
  y: number,
  s: number,
  seed: number,
): string {
  const r = rng(seed);
  const P = (px: number, py: number): Pt => [x + px * s, y + py * s];
  const louvers = [20, 25, 30, 35, 40]
    .map((py) =>
      line(P(12, py), P(58, py + 3), r, { roughness: 0.3, double: false }),
    )
    .join("");
  return [
    pencil(
      `M${n(P(18, 46)[0])} ${n(P(18, 46)[1])}L${n(P(16, 88)[0])} ${n(P(16, 88)[1])}M${n(P(52, 46)[0])} ${n(P(52, 46)[1])}L${n(P(54, 88)[0])} ${n(P(54, 88)[1])}`,
      "ink",
      2.4 * Math.sqrt(s),
    ),
    pencil(
      `M${n(P(24, 46)[0])} ${n(P(24, 46)[1])}L${n(P(23, 84)[0])} ${n(P(23, 84)[1])}M${n(P(46, 46)[0])} ${n(P(46, 46)[1])}L${n(P(47, 84)[0])} ${n(P(47, 84)[1])}`,
      "pencil",
      1.6 * Math.sqrt(s),
    ),
    path(cut([P(9, 14), P(61, 14), P(61, 47), P(9, 47)], r, 0.5), "white"),
    pencil(louvers, "pencil", 1.2 * Math.sqrt(s)),
    path(cut([P(4, 15), P(35, 4), P(66, 15)], r, 0.4), "white"),
    boil(
      (rr) =>
        pencil(
          outline([P(9, 14), P(61, 14), P(61, 47), P(9, 47)], rr, {
            roughness: 0.5,
            double: false,
          }) +
            outline([P(4, 15), P(35, 4), P(66, 15)], rr, {
              roughness: 0.5,
              double: false,
              closed: false,
            }),
          "ink",
          1.4 * Math.sqrt(s),
        ),
      seed,
    ),
  ].join("");
}

// Twin texts for the unit toggle: the stylesheet shows one of them.
export function unitText(
  x: number,
  y: number,
  metric: string,
  imperial: string,
  size: number,
  attrs = "",
  tone = "ink",
): string {
  const t = (cls: string, content: string) =>
    `<text class="cl-hand f-${tone} ${cls}" x="${x}" y="${y}" font-size="${size}"${attrs ? ` ${attrs}` : ""}>${content}</text>`;
  return metric === imperial
    ? t("", metric)
    : t("u-metric", metric) + t("u-imperial", imperial);
}

export function f1(value: number, decimals = 1): string {
  return value.toFixed(decimals);
}

export function toF(celsius: number): number {
  return (celsius * 9) / 5 + 32;
}

export function toFDelta(celsius: number): number {
  return (celsius * 9) / 5;
}

export function toIn(mm: number): number {
  return mm / 25.4;
}

// A small hand-drawn weather glyph centred on (cx, cy).
export function skyGlyph(
  kind: string,
  cx: number,
  cy: number,
  s: number,
  seed: number,
): string {
  const r = rng(seed);
  const sun = (sx: number, sy: number, rad: number) =>
    `<circle class="f-sun" cx="${n(sx)}" cy="${n(sy)}" r="${n(rad)}"/>` +
    pencil(
      Array.from({ length: 8 }, (_, i) => {
        const a = (i / 8) * Math.PI * 2 + 0.3;
        return `M${n(sx + Math.cos(a) * rad * 1.35)} ${n(sy + Math.sin(a) * rad * 1.35)}L${n(sx + Math.cos(a) * rad * 1.8)} ${n(sy + Math.sin(a) * rad * 1.8)}`;
      }).join(""),
      "sun-deep",
      1.6 * s,
    );
  const cloud = (sx: number, sy: number, k: number, tone: string) => {
    const pts: Pt[] = [
      [sx - 12 * k, sy + 5 * k],
      [sx - 13 * k, sy - 1 * k],
      [sx - 7 * k, sy - 5 * k],
      [sx - 3 * k, sy - 10 * k],
      [sx + 5 * k, sy - 9 * k],
      [sx + 9 * k, sy - 4 * k],
      [sx + 14 * k, sy - 1 * k],
      [sx + 13 * k, sy + 5 * k],
    ];
    return (
      path(curve(pts, true), tone) +
      pencil(curve(pts, true), "ink", 1.1 * s, 'opacity="0.7"')
    );
  };
  switch (kind) {
    case "sun":
      return sun(cx, cy, 7 * s);
    case "partly":
      return (
        sun(cx + 5 * s, cy - 4 * s, 6 * s) +
        cloud(cx - 2 * s, cy + 4 * s, 0.85 * s, "cloud")
      );
    case "cloud":
      return (
        cloud(cx - 4 * s, cy - 1 * s, 0.7 * s, "cloud-shade") +
        cloud(cx + 2 * s, cy + 3 * s, 0.9 * s, "cloud")
      );
    default:
      return (
        cloud(cx, cy - 4 * s, 0.95 * s, "rain-cloud") +
        pencil(
          [-7, 0, 7]
            .map(
              (dx) =>
                `M${n(cx + dx * s)} ${n(cy + 6 * s)}L${n(cx + (dx - 2) * s + between(r, -0.5, 0.5))} ${n(cy + 12 * s)}`,
            )
            .join(""),
          "rain",
          1.8 * s,
        )
      );
  }
}

export function rect(
  x: number,
  y: number,
  w: number,
  h: number,
  tone: string,
  seed: number,
  jitter = 0.6,
): string {
  return path(cutRect(x, y, w, h, rng(seed), jitter), tone);
}
