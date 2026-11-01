import type { ReportDay } from "../../reportChart";
import { cut, cutRect, torn, tornRect } from "../paper";
import { boil, path, pencil, type Scene } from "../scene";
import {
  f1,
  leafShape,
  skyGlyph,
  toF,
  toFDelta,
  toIn,
  unitText,
} from "../shapes";
import {
  between,
  brush,
  cloudShape,
  curve,
  ellipse,
  hatch,
  line,
  n,
  outline,
  type Pt,
  type Rnd,
  ring,
  rng,
} from "../sketch";
import { groundStrip } from "./doodles";

function fixed(value: number, decimals: number): string {
  const k = 10 ** decimals;
  const rounded = Math.round(value * k) / k;
  return Object.is(rounded, -0) ? "0" : String(rounded);
}

function deg(a: number): number {
  return (a * Math.PI) / 180;
}

function wobble(points: Pt[], rr: Rnd, by = 0.7): Pt[] {
  return points.map(
    ([x, y]) => [x + between(rr, -by, by), y + between(rr, -by, by)] as Pt,
  );
}

function arrowHead(tip: Pt, from: Pt, size = 6): string {
  const a = Math.atan2(tip[1] - from[1], tip[0] - from[0]);
  return [a + 2.6, a - 2.6]
    .map(
      (b) =>
        `M${n(tip[0])} ${n(tip[1])}L${n(tip[0] + size * Math.cos(b))} ${n(tip[1] + size * Math.sin(b))}`,
    )
    .join("");
}

// Two clotheslines of paper day tags, pinned on one by one and tinted by
// how far each day's mean sat from the running normal.
export function clotheslineScene(days: ReportDay[]): Scene {
  const W = 260;
  const H = 210;
  const left = 18;
  const right = 242;
  const perRow = 16;
  const lineY = [48, 118];
  const sag = 9;
  const yOn = (row: number, x: number) => {
    const t = (x - left) / (right - left);
    return lineY[row] + sag * 4 * t * (1 - t);
  };
  const tone = (a: number) =>
    a >= 3
      ? "orange"
      : a >= 1
        ? "orange-soft"
        : a > -1
          ? "paper"
          : a > -3
            ? "blue-soft"
            : "blue";
  const r = rng(1001);
  const step = (right - left - 14) / perRow;
  const rows = [days.slice(0, perRow), days.slice(perRow)];
  const tags = rows
    .flatMap((row, ri) =>
      row.map((d, i) => {
        const k = ri * perRow + i;
        const cx = left + 7 + step * (i + 0.5);
        const top = yOn(ri, cx);
        const tr = rng(1100 + k);
        const tag: Pt[] = [
          [cx - 5.4, top + 1.5],
          [cx + 5.4, top + 1.5],
          [cx + 5.6, top + 23],
          [cx - 5.6, top + 23],
        ];
        const card =
          path(cut(tag, tr, 0.6), tone(d.temp_avg - d.n96.temp_avg)) +
          pencil(
            outline(tag, tr, { roughness: 0.3, double: false }),
            "ink",
            0.8,
            'opacity="0.7"',
          ) +
          `<circle class="f-white" cx="${n(cx)}" cy="${n(top + 18)}" r="1.4" opacity="0.8"/>`;
        const peg =
          path(cutRect(cx - 1.7, top - 4.5, 3.4, 9.5, tr, 0.3), "kraft") +
          pencil(
            `M${n(cx)} ${n(top - 4)}L${n(cx)} ${n(top + 4.5)}`,
            "kraft-dark",
            0.8,
          );
        const origin = `transform-origin:${n(cx)}px ${n(top)}px`;
        return `<g class="cl-pin" style="--pd:${n(0.35 + k * 0.07)}s;${origin}"><g class="cl-flutter" style="--fl:${n(between(r, 1.1, 1.7))}s;--fd:${n(-between(r, 0, 1.5))}s;${origin}">${card}</g>${peg}</g>`;
      }),
    )
    .join("");
  const cord = (row: number) =>
    Array.from({ length: 13 }, (_, i) => {
      const x = left + ((right - left) * i) / 12;
      return [x, yOn(row, x)] as Pt;
    });
  const posts = [left, right]
    .map((x, i) =>
      [
        path(
          brush(
            [
              [x, 196],
              [x + (i ? 1 : -1), 110],
              [x, 26],
            ],
            7,
            rng(1010 + i),
            { taper: 0.04, wobble: 0.08 },
          ),
          "kraft-dark",
        ),
        path(cutRect(x - 9, 24, 18, 5, rng(1012 + i), 0.3), "kraft-dark"),
      ].join(""),
    )
    .join("");
  const lines = boil(
    (rr) =>
      pencil(curve(wobble(cord(0), rr, 0.5)), "ink", 1.2) +
      pencil(curve(wobble(cord(1), rr, 0.5)), "ink", 1.2),
    1002,
  );
  const dayLabel = (row: number, i: number, label: string) => {
    const x = left + 7 + step * (i + 0.5);
    return `<text class="cl-hand f-pencil" x="${n(x)}" y="${n(yOn(row, x) + 38)}" font-size="15" font-weight="600" text-anchor="middle">${label}</text>`;
  };
  const labels = [
    dayLabel(0, 0, "1"),
    rows[0].length > 1
      ? dayLabel(0, rows[0].length - 1, String(rows[0].length))
      : "",
    rows[1].length ? dayLabel(1, 0, String(perRow + 1)) : "",
    rows[1].length > 1
      ? dayLabel(1, rows[1].length - 1, String(days.length))
      : "",
  ].join("");
  const swatch = (x: number, tone: string, label: string, seed: number) =>
    path(cutRect(x, 170, 12, 18, rng(seed), 0.4), tone) +
    `<text class="cl-hand f-ink" x="${x + 17}" y="187" font-size="22" font-weight="700">${label}</text>`;
  const legend = `<g class="cl-pop" style="--pop-d:2.6s">${swatch(40, "orange", "milder", 1020)}${swatch(144, "blue-soft", "cooler", 1021)}</g>`;
  const gusts = [
    [
      [140, 14],
      [170, 9],
      [200, 15],
      [232, 9],
    ],
    [
      [96, 88],
      [126, 83],
      [156, 89],
    ],
  ]
    .map((pts, i) =>
      pencil(
        curve(pts as unknown as Pt[]),
        "pencil",
        1.4,
        `pathLength="1" style="--gd:${n(0.8 + i * 0.6)}s"`,
        "cl-gust",
      ),
    )
    .join("");
  return {
    w: W,
    h: H,
    layers: [
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: groundStrip(W, 192, 24, 1003) + gusts,
        className: "cl-base",
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: posts + lines,
        enter: { delay: 0, y: 20, kind: "grow" },
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: `<g class="cl-sway" style="transform-origin:130px 40px;--sway:3.2s">${tags}</g>${labels}${legend}`,
        className: "cl-flat",
      },
    ],
  };
}

function fern(
  cx: number,
  cy: number,
  angle: number,
  size: number,
  rr: Rnd,
): string {
  const branch = (
    x: number,
    y: number,
    a: number,
    len: number,
    depth: number,
  ): string => {
    const ex = x + len * Math.cos(deg(a));
    const ey = y + len * Math.sin(deg(a));
    let d = line([x, y], [ex, ey], rr, { roughness: 0.25, double: false });
    if (depth > 0) {
      const count = Math.max(2, Math.round(len / 6));
      for (let i = 1; i <= count; i++) {
        const t = i / (count + 1);
        const bx = x + (ex - x) * t;
        const by = y + (ey - y) * t;
        const sub = len * 0.42 * (1 - t * 0.6);
        d += branch(bx, by, a + 48, sub, depth - 1);
        d += branch(bx, by, a - 48, sub, depth - 1);
      }
    }
    return d;
  };
  return [-24, 0, 24]
    .map((da, i) => branch(cx, cy, angle + da, size * (i === 1 ? 1 : 0.72), 2))
    .join("");
}

// A window thermometer drops toward zero while frost ferns creep in from the
// corners of the pane; it stops just above, and the frost backs off again.
export function frostScene(lowest: number): Scene {
  const W = 240;
  const H = 200;
  const r = rng(1201);
  const frame: Pt[] = [
    [10, 14],
    [150, 14],
    [150, 180],
    [10, 180],
  ];
  const glassL: Pt[] = [
    [22, 26],
    [76, 26],
    [76, 168],
    [22, 168],
  ];
  const glassR: Pt[] = [
    [84, 26],
    [138, 26],
    [138, 168],
    [84, 168],
  ];
  const outside = [
    path(cut(glassL, r, 0.4), "sky"),
    path(cut(glassR, r, 0.4), "sky"),
    path(
      brush(
        [
          [18, 74],
          [52, 66],
          [92, 72],
          [128, 60],
        ],
        5,
        r,
        { taper: 0.4, wobble: 0.15 },
      ),
      "trunk",
    ),
    path(
      brush(
        [
          [70, 69],
          [86, 52],
          [100, 44],
        ],
        2.4,
        r,
        { taper: 0.6 },
      ),
      "trunk",
    ),
    leafShape(48, 78, 7, 20, "birch", r),
    leafShape(98, 52, 6, -40, "maple", r),
    leafShape(118, 72, 6.5, 30, "birch", r),
    pencil(
      hatch(glassL.slice(0, 3), r, { angle: 60, gap: 22, roughness: 0.3 }),
      "white",
      2.6,
      'opacity="0.45"',
    ),
    pencil(
      hatch(glassR.slice(0, 3), r, { angle: 60, gap: 26, roughness: 0.3 }),
      "white",
      2.2,
      'opacity="0.4"',
    ),
  ].join("");
  const ferns = (
    [
      [22, 168, -45, 1211],
      [76, 168, -135, 1212],
      [84, 168, -45, 1213],
      [138, 168, -135, 1214],
      [22, 26, 45, 1215],
      [138, 26, 135, 1216],
    ] as const
  )
    .map(([x, y, a, seed], i) => {
      const fr = rng(seed);
      const size = i < 4 ? 34 : 24;
      return `<g class="cl-fern" style="transform-origin:${x}px ${y}px;transform:scale(0.42);opacity:0.6;--fnd:${n(i * 0.12)}s">${pencil(fern(x, y, a, size, fr), "white", 1.3, 'opacity="0.95"')}${pencil(fern(x + 1, y - 1, a + 6, size * 0.8, fr), "glass", 0.9)}</g>`;
    })
    .join("");
  const wood = [
    path(
      cut(frame, r, 0.5) + cut(glassL, r, 0.3) + cut(glassR, r, 0.3),
      "white",
      undefined,
      'fill-rule="evenodd"',
    ),
    pencil(
      hatch(
        [
          [140, 16],
          [150, 16],
          [150, 178],
          [140, 178],
        ],
        r,
        { angle: 80, gap: 4, roughness: 0.3 },
      ),
      "wall-shade",
      1.4,
    ),
    boil(
      (rr) =>
        pencil(
          outline(frame, rr, { roughness: 0.4, double: false }) +
            outline(glassL, rr, { roughness: 0.3, double: false }) +
            outline(glassR, rr, { roughness: 0.3, double: false }),
          "ink",
          1.2,
          'opacity="0.8"',
        ),
      1202,
    ),
    path(cutRect(2, 178, 156, 10, r, 0.4), "kraft"),
    pencil(line([2, 188], [158, 188], r, { double: false }), "kraft-dark", 1.6),
  ].join("");
  const tMin = -10;
  const tMax = 20;
  const yT = (t: number) => 150 - ((t - tMin) * (150 - 38)) / (tMax - tMin);
  const tx = 176;
  const start = 14;
  const low = Math.max(tMin + 1, lowest);
  const column = 158 - yT(low);
  const body: Pt[] = [
    [164, 22],
    [199, 22],
    [201, 176],
    [162, 176],
  ];
  const ticks = Array.from({ length: (tMax - tMin) / 2 + 1 }, (_, i) => {
    const t = tMin + i * 2;
    const long = t % 10 === 0;
    return `M${n(tx - (long ? 9 : 6))} ${n(yT(t))}L${n(tx - 3.5)} ${n(yT(t))}`;
  }).join("");
  const numbers = [20, 10, 0, -10]
    .map(
      (t) =>
        `<text class="cl-print f-ink" x="${tx + 6}" y="${n(yT(t) + 3.4)}" font-size="9.5" font-weight="700">${t}</text>`,
    )
    .join("");
  const thermometer = [
    path(cut(body, r, 0.4), "paper"),
    pencil(outline(body, r, { roughness: 0.3, double: false }), "ink", 1.1),
    pencil(ticks, "ink", 0.9),
    numbers,
    pencil(
      `M${tx - 10} ${n(yT(0))}L${tx + 4} ${n(yT(0))}`,
      "blue",
      1.2,
      'stroke-dasharray="2 2"',
    ),
    path(cutRect(tx - 3, 34, 6, 126, r, 0.2), "glass"),
    `<g class="cl-liquid" style="transform-origin:${tx}px 158px;--lq:${fixed((158 - yT(start)) / column, 3)}"><rect class="f-stamp" x="${tx - 1.6}" y="${n(yT(low))}" width="3.2" height="${n(column)}"/></g>`,
    `<circle class="f-stamp" cx="${tx}" cy="161" r="5.5"/>`,
    pencil(
      ellipse(tx, 161, 5.6, 5.6, r, { roughness: 0.4, double: false }),
      "ink",
      1,
    ),
    `<circle class="f-white" cx="${tx - 1.8}" cy="159" r="1.4" opacity="0.8"/>`,
  ].join("");
  const ly = yT(low);
  const label = `<g class="cl-pop" style="--pop-d:2.7s">${pencil(`M${tx + 3} ${n(ly)}L205 ${n(ly - 6)}`, "ink", 1, 'stroke-dasharray="2 2"')}${unitText(206, ly - 14, "lowest", "lowest", 13, "", "pencil")}${unitText(206, ly + 4, `${f1(low)}°`, `${f1(toF(low))}°`, 19, 'font-weight="700"')}</g>`;
  return {
    w: W,
    h: H,
    layers: [
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: outside,
        className: "cl-base",
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: ferns,
        className: "cl-flat",
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: wood,
        enter: { delay: 0, y: 14, kind: "fade" },
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: thermometer,
        enter: { delay: 0.15, y: -30, rotate: 4, kind: "drop" },
      },
      { x: 0, y: 0, w: W, h: H, svg: label, className: "cl-flat" },
    ],
  };
}

// Height marks on a door frame, one for each 30-year normal, ruled in by a
// pencil from the oldest up to the newest.
export function doorframeScene(n61: number, n91: number, n96: number): Scene {
  const W = 240;
  const H = 210;
  const r = rng(1301);
  const jamb = [150, 176];
  const low = Math.min(n61, n91, n96);
  const span = Math.max(0.3, Math.max(n61, n91, n96) - low);
  const k = 76 / span;
  const base = 158;
  const yOf = (t: number) => base - (t - low) * k;
  const marks = [
    { label: "1961–1990", t: n61, seed: 1311 },
    { label: "1991–2020", t: n91, seed: 1312 },
    { label: "1996–2025", t: n96, seed: 1313 },
  ];
  const stripes = Array.from({ length: 12 }, (_, i) => {
    const x = 6 + i * 12;
    return line([x, 0], [x + between(r, -1, 1), 186], r, {
      roughness: 0.3,
      double: false,
    });
  }).join("");
  const room = [
    `<rect class="f-wall" x="0" y="0" width="${jamb[0]}" height="190"/>`,
    pencil(stripes, "wall-shade", 2.4, 'opacity="0.8"'),
    path(cutRect(jamb[1], 0, W - jamb[1], 190, r, 0.5), "sky-deep"),
    path(
      cut(
        [
          [222, 10],
          [244, 4],
          [244, 200],
          [222, 186],
        ],
        r,
        0.5,
      ),
      "white",
    ),
    pencil(
      hatch(
        [
          [224, 14],
          [242, 8],
          [242, 196],
          [224, 184],
        ],
        r,
        { angle: 80, gap: 6, roughness: 0.3 },
      ),
      "cloud-shade",
      1.6,
    ),
    `<circle class="f-sun" cx="227" cy="104" r="2.4"/>`,
    path(cutRect(-4, 186, W + 8, 30, r, 0.4), "kraft-dark"),
    pencil(
      [60, 132, 196]
        .map((x) => line([x, 188], [x - 8, 210], r, { double: false }))
        .join(""),
      "kraft",
      1.4,
    ),
    path(cutRect(0, 178, jamb[0], 9, r, 0.3), "white"),
  ].join("");
  const grain = Array.from({ length: 4 }, (_, i) =>
    line(
      [jamb[0] + 5 + i * 5.5, 4],
      [jamb[0] + 4 + i * 5.5 + between(r, -1, 1), 184],
      r,
      {
        roughness: 0.6,
        double: false,
      },
    ),
  ).join("");
  const frame = [
    path(cutRect(jamb[0], -2, jamb[1] - jamb[0], 190, r, 0.4), "kraft"),
    pencil(grain, "kraft-dark", 1.1, 'opacity="0.8"'),
    boil(
      (rr) =>
        pencil(
          line([jamb[0], -2], [jamb[0], 188], rr, {
            roughness: 0.4,
            double: false,
          }) +
            line([jamb[1], -2], [jamb[1], 188], rr, {
              roughness: 0.4,
              double: false,
            }),
          "ink",
          1.3,
        ),
      1302,
    ),
  ].join("");
  const T = 3.9;
  const ticks = marks
    .map((m, i) => {
      const y = yOf(m.t);
      const d = 0.6 + i * 1.05;
      const mr = rng(m.seed);
      return `<path class="s-ink cl-draw" d="${line([jamb[0] - 3, y + 0.6], [jamb[1] + 3, y - 0.6], mr, { roughness: 0.3, double: false })}" fill="none" stroke-width="2" stroke-linecap="round" pathLength="1" style="--draw:0.45s;--draw-d:${n(d)}s"/>`;
    })
    .join("");
  const labels = marks
    .map((m, i) => {
      const y = yOf(m.t);
      const decimals = Math.abs(m.t * 10 - Math.round(m.t * 10)) > 0.01 ? 2 : 1;
      return `<g class="cl-pop" style="--pop-d:${n(1.05 + i * 1.05)}s">${`<text class="cl-hand f-pencil" x="10" y="${n(y + 5)}" font-size="14" font-weight="600">${m.label}</text>`}${unitText(144, y + 5.5, `${f1(m.t, decimals)}°`, `${f1(toF(m.t), decimals)}°`, 17, 'font-weight="700" text-anchor="end"')}</g>`;
    })
    .join("");
  const top = yOf(n96);
  const bottom = yOf(n61);
  const rise = `<g class="cl-pop" style="--pop-d:3.75s">${pencil(
    `M185 ${n(bottom - 2)}L185 ${n(top + 4)}` +
      arrowHead([185, top + 2], [185, bottom], 6),
    "white",
    1.6,
  )}${unitText(190, (top + bottom) / 2 + 6, `+${f1(n96 - n61)}°`, `+${f1(toFDelta(n96 - n61))}°`, 15, 'font-weight="700"', "white")}</g>`;
  const at = (t: number) => n((t / T) * 100);
  const pose = (x: number, y: number) =>
    `transform:translate(${n(x)}px,${n(y)}px)`;
  const keys = [
    `0%{${pose(232, 30)}}`,
    ...marks.flatMap((m, i) => {
      const y = yOf(m.t);
      const d = 0.6 + i * 1.05;
      return [
        `${at(d - 0.25)}%{${pose(jamb[0] - 6, y - 10)}}`,
        `${at(d)}%{${pose(jamb[0] - 3, y)}}`,
        `${at(d + 0.45)}%{${pose(jamb[1] + 3, y)}}`,
        `${at(d + 0.62)}%{${pose(jamb[1] + 10, y - 12)}}`,
      ];
    }),
    `100%{${pose(232, yOf(n96) - 34)}}`,
  ].join("");
  const pr = rng(1320);
  const pencilShape = `<g class="cl-door-pencil" style="${pose(232, yOf(n96) - 34)}">${[
    path(
      cut(
        [
          [0, 0],
          [6, -9],
          [11, -6],
        ],
        pr,
        0.2,
      ),
      "skin",
    ),
    path(
      cut(
        [
          [1.6, -2.4],
          [3.2, -4.8],
          [5.8, -3.2],
        ],
        pr,
        0.1,
      ),
      "ink",
    ),
    path(
      cut(
        [
          [6, -9],
          [11, -6],
          [42, -56],
          [37, -59],
        ],
        pr,
        0.3,
      ),
      "birch",
    ),
    pencil(
      line([8.5, -7.5], [39.5, -57.5], pr, { double: false }),
      "sun-deep",
      1.2,
    ),
    path(
      cut(
        [
          [37, -59],
          [42, -56],
          [45, -61],
          [40, -64],
        ],
        pr,
        0.2,
      ),
      "granite",
    ),
    path(
      cut(
        [
          [40, -64],
          [45, -61],
          [48, -66],
          [43, -69],
        ],
        pr,
        0.2,
      ),
      "orange-soft",
    ),
    pencil(
      outline(
        [
          [0, 0],
          [11, -6],
          [48, -66],
          [43, -69],
          [6, -9],
        ],
        pr,
        { roughness: 0.3, double: false },
      ),
      "ink",
      1,
    ),
  ].join("")}</g>`;
  return {
    w: W,
    h: H,
    layers: [
      { x: 0, y: 0, w: W, h: H, svg: room, className: "cl-base" },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: frame,
        enter: { delay: 0, y: 12, kind: "fade" },
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: ticks + labels + rise,
        className: "cl-flat",
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: `<style>@keyframes cl-door-pencil{${keys}}</style>${pencilShape}`,
        className: "cl-flat",
      },
    ],
  };
}

// A puddle on the park path that spreads day by day with the month's rain,
// short of the dashed outline of an ordinary month's puddle, while drizzle
// keeps ringing its surface.
export function puddleScene(days: ReportDay[], normal: number): Scene {
  const W = 250;
  const H = 200;
  const r = rng(1401);
  const cx = 124;
  const cy = 146;
  const total = days.at(-1)?.precip_cum ?? 0;
  const scale = Math.sqrt(Math.max(total, 0.1) / normal);
  const blob = ring(cx, cy, 96, 31, 26).map(([x, y], i) => {
    const k = 1 + 0.08 * Math.sin(i * 1.7) + between(r, -0.04, 0.04);
    return [cx + (x - cx) * k, cy + (y - cy) * k] as Pt;
  });
  const shrink = (pts: Pt[], s: number) =>
    pts.map(([x, y]) => [cx + (x - cx) * s, cy + (y - cy) * s] as Pt);
  const puddle = shrink(blob, scale);
  const ground = [
    path(
      tornRect(-6, 92, W + 12, 120, r, {
        amp: 2.4,
        torn: [true, false, false, false],
      }),
      "path",
    ),
    pencil(
      hatch(
        [
          [0, 100],
          [W, 100],
          [W, H],
          [0, H],
        ],
        r,
        { angle: 12, gap: 8, roughness: 0.6 },
      ),
      "kraft-dark",
      1.2,
      'opacity="0.45"',
    ),
    ...Array.from({ length: 14 }, () => {
      const x = between(r, 8, W - 8);
      const y = between(r, 104, 196);
      if (Math.abs((x - cx) / 100) ** 2 + Math.abs((y - cy) / 36) ** 2 < 1)
        return "";
      return `<ellipse class="f-granite" cx="${n(x)}" cy="${n(y)}" rx="${n(between(r, 1.6, 3.2))}" ry="${n(between(r, 1, 2))}"/>`;
    }),
    leafShape(30, 118, 7, 60, "maple", r),
    leafShape(222, 112, 6.5, -30, "birch", r),
    leafShape(20, 184, 7, 110, "rowan", r),
    pencil(
      curve(blob, true),
      "slate",
      1.6,
      'stroke-dasharray="5 4" opacity="0.9"',
    ),
  ].join("");
  const keyframes = days
    .map(
      (d, i) =>
        `${n(((i + 1) / days.length) * 100)}%{transform:scale(${fixed(Math.sqrt(Math.max(d.precip_cum, 0.05) / Math.max(total, 0.1)), 3)})}`,
    )
    .join("");
  const water = `<g class="cl-puddle" style="transform-origin:${cx}px ${cy}px">${[
    path(torn(puddle, r, { amp: 1.2, step: 6 }), "water"),
    pencil(
      hatch(shrink(puddle, 0.8).slice(13), r, {
        angle: -8,
        gap: 6,
        roughness: 0.4,
      }),
      "water-deep",
      1.6,
      'opacity="0.5"',
    ),
    pencil(
      curve([
        [cx - 52 * scale, cy - 10 * scale],
        [cx - 10 * scale, cy - 16 * scale],
        [cx + 30 * scale, cy - 14 * scale],
      ]),
      "white",
      2.6,
      'opacity="0.75"',
    ),
    boil(
      (rr) =>
        pencil(
          curve(wobble(puddle, rr, 0.6), true),
          "ink",
          1.1,
          'opacity="0.6"',
        ),
      1402,
    ),
  ].join("")}</g>`;
  const ripples = (
    [
      [cx - 44 * scale, cy + 6 * scale, 0],
      [cx + 30 * scale, cy - 4 * scale, 0.9],
      [cx - 8 * scale, cy + 14 * scale, 1.7],
      [cx + 58 * scale, cy + 10 * scale, 2.4],
    ] as const
  )
    .map(
      ([x, y, d]) =>
        `<g class="cl-ripple" style="transform-origin:${n(x)}px ${n(y)}px;--rd:${n(d)}s"><ellipse class="s-white" cx="${n(x)}" cy="${n(y)}" rx="9" ry="3" fill="none" stroke-width="1.4"/><ellipse class="s-white" cx="${n(x)}" cy="${n(y)}" rx="4.5" ry="1.5" fill="none" stroke-width="1.2" opacity="0.8"/></g>`,
    )
    .join("");
  const br = rng(1410);
  const bx = cx - 20;
  const by = cy - 2;
  const boat = [
    path(
      cut(
        [
          [bx - 20, by - 8],
          [bx + 20, by - 8],
          [bx + 13, by + 2],
          [bx - 13, by + 2],
        ],
        br,
        0.4,
      ),
      "paper",
    ),
    path(
      cut(
        [
          [bx - 11, by - 8],
          [bx, by - 26],
          [bx + 11, by - 8],
        ],
        br,
        0.4,
      ),
      "fiber",
    ),
    pencil(
      [-6, -2, 2, 6]
        .map(
          (dx) =>
            `M${n(bx + dx - 3)} ${n(by - 5)}L${n(bx + dx + 1)} ${n(by - 5)}`,
        )
        .join("") +
        `M${n(bx - 4)} ${n(by - 14)}L${n(bx + 3)} ${n(by - 14)}M${n(bx - 2)} ${n(by - 18)}L${n(bx + 2)} ${n(by - 18)}`,
      "pencil",
      0.9,
    ),
    pencil(
      outline(
        [
          [bx - 20, by - 8],
          [bx - 11, by - 8],
          [bx, by - 26],
          [bx + 11, by - 8],
          [bx + 20, by - 8],
          [bx + 13, by + 2],
          [bx - 13, by + 2],
        ],
        br,
        { roughness: 0.4, double: false },
      ),
      "ink",
      1.1,
    ),
  ].join("");
  const cr = rng(1420);
  const cloudPts = cloudShape(6, 4, 88, 34, cr, 4);
  const cloud = [
    path(torn(cloudPts, cr, { amp: 1.6 }), "rain-cloud"),
    pencil(
      hatch(
        cloudPts.filter((p) => p[1] > 24),
        cr,
        { angle: -28, gap: 5, roughness: 0.4 },
      ),
      "rain-cloud-shade",
      2,
    ),
    ...[18, 34, 50, 66, 80].map(
      (x, i) =>
        `<g class="cl-drizzle" style="--zd:${n(i * 0.23)}s">${pencil(`M${x} 44L${x - 2} 52`, "rain", 1.8)}</g>`,
    ),
  ].join("");
  const totalLabel = `<g class="cl-pop" style="--pop-d:4.7s">${unitText(150, 58, `${f1(total)} mm`, `${f1(toIn(total), 2)} in`, 21, 'font-weight="700"')}${pencil(
    curve([
      [176, 66],
      [182, 82],
      [cx + 70 * scale, cy - 20 * scale],
    ]),
    "ink",
    1.3,
  )}${pencil(arrowHead([cx + 70 * scale, cy - 20 * scale], [182, 82]), "ink", 1.3)}</g>`;
  const normalLabel = `<g class="cl-pop" style="--pop-d:0.7s">${unitText(244, 194, `normal ${f1(normal)} mm`, `normal ${f1(toIn(normal), 2)} in`, 14, 'font-weight="600" text-anchor="end"', "slate")}</g>`;
  return {
    w: W,
    h: H,
    layers: [
      { x: 0, y: 0, w: W, h: H, svg: ground, className: "cl-base" },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: `<style>@keyframes cl-puddle-grow{0%{transform:scale(0.04)}${keyframes}}</style>${water}${ripples}`,
        className: "cl-flat",
      },
      {
        x: bx - 24,
        y: by - 30,
        w: 48,
        h: 36,
        svg: `<g transform="translate(${-(bx - 24)} ${-(by - 30)})">${boat}</g>`,
        loop: "cl-rock",
        enter: { delay: 4.4, y: -30, kind: "drop" },
      },
      {
        x: 14,
        y: 8,
        w: 100,
        h: 60,
        svg: cloud,
        loop: "cl-drift",
        enter: { delay: 0.1, x: -40, kind: "place" },
        vars: { drift: "7s", dx: "70%" },
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: totalLabel + normalLabel,
        className: "cl-flat",
      },
    ],
  };
}

type Limb = [number, number];

// A jogger in a light jacket running in place along the shore path, breath
// puffing in the cool air.
export function joggerScene(feels: number): Scene {
  const W = 260;
  const H = 200;
  const hip: Pt = [104, 116];
  const shoulder: Pt = [111, 80];
  const joint = (from: Pt, len: number, a: number): Pt => [
    from[0] + len * Math.sin(deg(a)),
    from[1] + len * Math.cos(deg(a)),
  ];
  const limb = (from: Pt, l1: number, l2: number, [a1, a2]: Limb): Pt[] => {
    const mid = joint(from, l1, a1);
    return [from, mid, joint(mid, l2, a2)];
  };
  const poses: {
    legs: [Limb, Limb];
    arms: [Limb, Limb];
    dy: number;
    tail: number;
  }[] = [
    {
      legs: [
        [40, -4],
        [-30, -80],
      ],
      arms: [
        [-38, 46],
        [36, 118],
      ],
      dy: -2.5,
      tail: -6,
    },
    {
      legs: [
        [8, -14],
        [18, -96],
      ],
      arms: [
        [-10, 72],
        [12, 98],
      ],
      dy: 1.5,
      tail: 4,
    },
    {
      legs: [
        [-30, -80],
        [40, -4],
      ],
      arms: [
        [36, 118],
        [-38, 46],
      ],
      dy: -2.5,
      tail: -6,
    },
    {
      legs: [
        [18, -96],
        [8, -14],
      ],
      arms: [
        [12, 98],
        [-10, 72],
      ],
      dy: 1.5,
      tail: 4,
    },
  ];
  const frames = poses
    .map((p, i) => {
      const rr = rng(1500 + i);
      const h: Pt = [hip[0], hip[1] + p.dy];
      const s: Pt = [shoulder[0], shoulder[1] + p.dy];
      const [legA, legB] = p.legs.map((l) => limb(h, 25, 24, l));
      const [armA, armB] = p.arms.map((a) =>
        limb([s[0] - 1, s[1] + 3], 16, 15, a),
      );
      const shoe = (foot: Pt) =>
        `<ellipse class="f-orange" cx="${n(foot[0] + 3)}" cy="${n(foot[1])}" rx="6" ry="3" transform="rotate(${n(between(rr, -8, 8))} ${n(foot[0] + 3)} ${n(foot[1])})"/>`;
      const leg = (pts: Pt[], back: boolean) =>
        pencil(curve(pts), back ? "pencil" : "ink", 5.4) + shoe(pts[2]);
      const arm = (pts: Pt[], back: boolean) =>
        pencil(curve(pts), back ? "copper-dark" : "copper", 5.6) +
        `<circle class="f-skin" cx="${n(pts[2][0])}" cy="${n(pts[2][1])}" r="3"/>`;
      const torso: Pt[] = [
        [s[0] - 11, s[1] - 2],
        [s[0] + 9, s[1] - 3],
        [h[0] + 9, h[1] + 1],
        [h[0] - 10, h[1] + 2],
      ];
      const head: Pt = [s[0] + 5, s[1] - 15];
      const tail = curve([
        [head[0] - 8, head[1] - 4],
        [head[0] - 15, head[1] + p.tail * 0.4],
        [head[0] - 21, head[1] + 2 + p.tail],
      ]);
      return `<g class="cl-fr cl-fr${i}">${[
        arm(armB, true),
        leg(legB, true),
        pencil(tail, "trunk", 4.4),
        path(cut(torso, rr, 0.5), "copper"),
        pencil(
          outline(torso, rr, { roughness: 0.4, double: false }),
          "ink",
          1.1,
        ),
        pencil(
          `M${n(s[0] - 1)} ${n(s[1])}L${n(h[0] - 1)} ${n(h[1])}`,
          "copper-dark",
          1.2,
        ),
        path(cutRect(h[0] - 10, h[1] - 2, 19, 7, rr, 0.3), "ink"),
        leg(legA, false),
        `<circle class="f-skin" cx="${n(head[0])}" cy="${n(head[1])}" r="10"/>`,
        path(
          cut(
            [
              [head[0] - 10, head[1] - 2],
              ...Array.from({ length: 7 }, (_, j) => {
                const a = Math.PI + (j / 6) * Math.PI;
                return [
                  head[0] + 10.5 * Math.cos(a),
                  head[1] - 2 + 10.5 * Math.sin(a),
                ] as Pt;
              }),
            ],
            rr,
            0.3,
          ),
          "trunk",
        ),
        path(cutRect(head[0] - 10.5, head[1] - 5, 21, 4.5, rr, 0.3), "stamp"),
        `<circle class="f-ink" cx="${n(head[0] + 5)}" cy="${n(head[1] + 1)}" r="1.2"/>`,
        `<ellipse class="f-orange-soft" cx="${n(head[0] + 4)}" cy="${n(head[1] + 5)}" rx="2.6" ry="1.6" opacity="0.85"/>`,
        pencil(`M${n(head[0] + 6)} ${n(head[1] + 6)}l3 0.6`, "ink", 1.2),
        arm(armA, false),
      ].join("")}</g>`;
    })
    .join("");
  const mouth: Pt = [shoulder[0] + 15, shoulder[1] - 9];
  const puffs = [0, 1, 2]
    .map((i) => {
      const pr = rng(1520 + i);
      const blobs = [
        [0, 0, 4.5],
        [5, -2, 3.6],
        [3, 3, 3.2],
      ]
        .map(
          ([dx, dy, rad]) =>
            `<circle class="f-cloud" cx="${n(mouth[0] + 6 + dx)}" cy="${n(mouth[1] + dy)}" r="${n(rad + between(pr, -0.4, 0.4))}"/>`,
        )
        .join("");
      return `<g class="cl-puff" style="transform-origin:${n(mouth[0] + 6)}px ${n(mouth[1])}px;--pfd:${n(i * 0.62)}s;transform:translate(${i * 11}px,${-i * 4}px) scale(${n(0.6 + i * 0.3)});opacity:${n(1 - i * 0.3)}">${blobs}${pencil(ellipse(mouth[0] + 8, mouth[1], 7, 5, pr, { double: false }), "pencil", 0.8, 'opacity="0.6"')}</g>`;
    })
    .join("");
  const r = rng(1530);
  const dashes = Array.from({ length: 8 }, (_, i) => {
    const x = i * 48 + 8;
    return `M${x} 182L${x + 14} 182M${x + 26} 190L${x + 34} 190`;
  }).join("");
  const tufts = Array.from({ length: 8 }, (_, i) => {
    const x = i * 48 + 30;
    return `M${x} 170l-2 -6M${x + 3} 170l1 -8M${x + 6} 170l3 -5`;
  }).join("");
  const ground = [
    path(
      tornRect(-6, 148, W + 12, 30, r, {
        amp: 1.6,
        torn: [true, false, false, false],
      }),
      "water",
    ),
    pencil(
      hatch(
        [
          [0, 152],
          [W, 152],
          [W, 168],
          [0, 168],
        ],
        r,
        { angle: -4, gap: 6, roughness: 0.4 },
      ),
      "water-deep",
      1.4,
      'opacity="0.6"',
    ),
    path(
      tornRect(-6, 166, W + 12, 40, r, {
        amp: 2,
        torn: [true, false, false, false],
      }),
      "path",
    ),
    `<clipPath id="cl-jog-ground"><rect width="${W}" height="${H}"/></clipPath><g clip-path="url(#cl-jog-ground)"><g class="cl-wave" style="--wl:-48px">${pencil(dashes, "kraft-dark", 1.6)}${pencil(tufts, "grass-dark", 1.4)}</g></g>`,
  ].join("");
  const speed = pencil(
    ["M66 92L82 92", "M58 108L78 108", "M64 124L80 124"].join(""),
    "pencil",
    1.6,
    'opacity="0.8"',
  );
  const label = unitText(
    160,
    46,
    `feels ${f1(feels)}°`,
    `feels ${f1(toF(feels))}°`,
    24,
    'font-weight="700" transform="rotate(-5 160 46)"',
  );
  return {
    w: W,
    h: H,
    layers: [
      { x: 0, y: 0, w: W, h: H, svg: ground, className: "cl-base" },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg:
          boil(
            (rr) =>
              pencil(
                line([62, 100], [80, 100], rr, { double: false }),
                "pencil",
                1.2,
              ),
            1531,
          ) + speed,
        className: "cl-flat",
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: frames,
        enter: { delay: 0.1, x: -30, kind: "place" },
      },
      { x: 0, y: 0, w: W, h: H, svg: puffs, className: "cl-flat" },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: `<g class="cl-write" style="--write:0.9s">${label}</g>`,
        className: "cl-flat",
        enter: { delay: 0.5, kind: "fade" },
      },
    ],
  };
}

// A garden sundial under the low October sun: the gnomon's shadow sweeps the
// dial and each hour's band darkens by that hour's average sunshine.
export function sundialScene(profile: number[], total: number): Scene {
  const W = 240;
  const H = 200;
  const r = rng(1601);
  const cx = 120;
  const cy = 120;
  const R = 74;
  const squash = 0.4;
  const noon = 13;
  const first = 8;
  const last = 18;
  const az = (h: number) => 15 * (h - noon);
  const onDial = (radius: number, a: number): Pt => [
    cx - radius * Math.sin(deg(a)),
    cy + radius * Math.cos(deg(a)) * squash,
  ];
  const plate = ring(cx, cy, R, R * squash, 36);
  const rim = ring(cx, cy + 6, R, R * squash, 36);
  const dial = [
    path(cut(rim, r, 0.4), "copper-dark"),
    path(cut(plate, r, 0.4), "copper"),
    pencil(
      hatch(plate.slice(18), r, { angle: -10, gap: 5, roughness: 0.4 }),
      "copper-dark",
      1.4,
      'opacity="0.55"',
    ),
    boil((rr) => pencil(curve(wobble(plate, rr, 0.6), true), "ink", 1.3), 1602),
  ].join("");
  const sweep = 4.4;
  const bands = Array.from({ length: last - first }, (_, i) => {
    const h = first + i;
    const f = profile[h] ?? 0;
    const a0 = az(h);
    const a1 = az(h + 1);
    const pts = [
      ...Array.from({ length: 5 }, (_, k) =>
        onDial(R * 0.92, a0 + ((a1 - a0) * k) / 4),
      ),
      ...Array.from({ length: 5 }, (_, k) =>
        onDial(R * 0.74, a1 - ((a1 - a0) * k) / 4),
      ),
    ];
    const d = 0.6 + (i / (last - first)) * sweep;
    return `<g class="cl-pop" style="--pop-d:${n(d + 0.15)}s">${path(cut(pts, r, 0.3), "sun", undefined, `opacity="${fixed(Math.min(0.95, 0.12 + 1.8 * f), 2)}"`)}</g>`;
  }).join("");
  const ticks = Array.from({ length: last - first + 1 }, (_, i) => {
    const a = az(first + i);
    const [x0, y0] = onDial(R * 0.7, a);
    const [x1, y1] = onDial(R * 0.96, a);
    return `M${n(x0)} ${n(y0)}L${n(x1)} ${n(y1)}`;
  }).join("");
  const numerals = [9, 12, 15, 18]
    .map((h) => {
      const [x, y] = onDial(R * 1.12, az(h));
      return `<text class="cl-print f-ink" x="${n(x)}" y="${n(y + 4.5)}" font-size="11" font-weight="700" text-anchor="middle">${h}</text>`;
    })
    .join("");
  const shadow = `<g transform="translate(${cx} ${cy}) scale(1 ${squash})"><g class="cl-dial-shadow"><path class="f-ink" d="M-4 0L0 ${n(R * 0.86)}L4 0Z" opacity="0.38"/></g></g>`;
  const gr = rng(1610);
  const gnomon = [
    path(
      cut(
        [
          [cx - 5, cy - 12],
          [cx + 7, cy + 16],
          [cx + 7, cy - 22],
        ],
        gr,
        0.3,
      ),
      "copper-dark",
    ),
    pencil(
      outline(
        [
          [cx - 5, cy - 12],
          [cx + 7, cy + 16],
          [cx + 7, cy - 22],
        ],
        gr,
        { roughness: 0.3, double: false },
      ),
      "ink",
      1.1,
    ),
  ].join("");
  const pedestal = [
    path(
      cut(
        [
          [cx - 22, cy + 30],
          [cx + 22, cy + 30],
          [cx + 28, 186],
          [cx - 28, 186],
        ],
        r,
        0.5,
      ),
      "granite",
    ),
    pencil(
      hatch(
        [
          [cx + 8, cy + 32],
          [cx + 22, cy + 32],
          [cx + 27, 184],
          [cx + 10, 184],
        ],
        r,
        { angle: 80, gap: 4, roughness: 0.3 },
      ),
      "granite-dark",
      1.4,
    ),
    path(cutRect(cx - 34, 182, 68, 9, r, 0.4), "granite-dark"),
    path(cutRect(cx - 22, cy + 47, 44, 20, rng(1611), 0.3), "kraft"),
    `<text class="cl-hand f-ink" x="${cx}" y="${cy + 62}" font-size="14" font-weight="700" text-anchor="middle">${f1(total)} h</text>`,
    boil(
      (rr) =>
        pencil(
          outline(
            [
              [cx - 22, cy + 30],
              [cx - 28, 186],
              [cx + 28, 186],
              [cx + 22, cy + 30],
            ],
            rr,
            { roughness: 0.4, double: false, closed: false },
          ),
          "ink",
          1.2,
        ),
      1612,
    ),
  ].join("");
  const sunAt = (h: number): Pt => {
    const t = (h - first) / (last - first);
    return [26 + t * 188, 58 - Math.sin(t * Math.PI) * 26];
  };
  const [sx, sy] = sunAt(noon);
  const sunKeys = Array.from({ length: 11 }, (_, i) => {
    const [x, y] = sunAt(first + ((last - first) * i) / 10);
    return `${i * 10}%{transform:translate(${n(x - sx)}px,${n(y - sy)}px)}`;
  }).join("");
  const shadowKeys = `0%{transform:rotate(${az(first)}deg)}100%{transform:rotate(${az(last)}deg)}`;
  const sun = `<g class="cl-dial-sun">${skyGlyph("sun", sx, sy, 1.15, 1620)}</g>`;
  return {
    w: W,
    h: H,
    layers: [
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg:
          groundStrip(W, 172, 34, 1603, "grass") +
          leafShape(36, 182, 7, 70, "maple", r) +
          leafShape(206, 188, 6, -20, "birch", r),
        className: "cl-base",
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: `<style>@keyframes cl-dial-sun{${sunKeys}}</style>${sun}`,
        className: "cl-flat",
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: `<style>@keyframes cl-dial-shadow{${shadowKeys}}</style>${pedestal}${dial}${bands}${pencil(ticks, "ink", 1)}${numerals}${shadow}${gnomon}`,
        enter: { delay: 0, y: 20, kind: "drop" },
      },
    ],
  };
}

const SECTORS = ["N", "NE", "E", "SE", "S", "SW", "W", "NW"] as const;

// A rooster weathervane on a roof ridge, swinging between the month's most
// common wind directions and lingering on each by its share of the time.
export function weathervaneScene(shares: Record<string, number>): Scene {
  const W = 240;
  const H = 200;
  const r = rng(1701);
  const cx = 120;
  const pivot = 62;
  const armsY = 106;
  const ranked = SECTORS.map((s, i) => ({
    s,
    a: i * 45,
    share: shares[s] ?? 0,
  })).sort((p, q) => q.share - p.share);
  const [first, second] = ranked;
  const stops = ranked.slice(0, 4).sort((p, q) => p.a - q.a);
  const facing = (a: number) => {
    const v = Math.sin(deg(a));
    return Math.abs(v) < 0.1 ? (v < 0 ? -0.1 : 0.1) : v;
  };
  const route: { a: number; hold: number }[] = [];
  const unwrap = (from: number, to: number) => {
    let d = to - from;
    while (d > 180) d -= 360;
    while (d < -180) d += 360;
    return from + d;
  };
  let angle = stops[0].a;
  for (const stop of [...stops, ...[...stops].reverse().slice(1)]) {
    const target = unwrap(angle, stop.a);
    const steps = Math.max(1, Math.round(Math.abs(target - angle) / 15));
    for (let i = 1; i <= steps; i++)
      route.push({ a: angle + ((target - angle) * i) / steps, hold: 0.25 });
    angle = target;
    route.push({ a: angle, hold: stop.share / 4 });
  }
  const last = unwrap(angle, first.a);
  const tail = Math.max(1, Math.round(Math.abs(last - angle) / 15));
  for (let i = 1; i <= tail; i++)
    route.push({ a: angle + ((last - angle) * i) / tail, hold: 0.25 });
  const totalTime = route.reduce((sum, p) => sum + p.hold, 0);
  let t = 0;
  const keys = [`0%{transform:scaleX(${fixed(facing(route[0].a), 2)})}`];
  for (const p of route) {
    t += p.hold;
    keys.push(
      `${fixed((100 * t) / totalTime, 2)}%{transform:scaleX(${fixed(facing(p.a), 2)})}`,
    );
  }
  const roof = [
    path(
      cut(
        [
          [-10, 214],
          [cx, 150],
          [W + 10, 214],
        ],
        r,
        0.6,
      ),
      "rowan",
    ),
    pencil(
      [162, 174, 186, 198]
        .map((y) => {
          const half = ((y - 150) / 64) * 130;
          return line([cx - half, y], [cx + half, y], r, {
            roughness: 0.4,
            double: false,
          });
        })
        .join(""),
      "maple",
      1.6,
    ),
    pencil(
      [-90, -60, -30, 0, 30, 60, 90]
        .map((dx) =>
          line([cx + dx * 0.15, 154], [cx + dx * 1.2, 210], r, {
            roughness: 0.3,
            double: false,
          }),
        )
        .join(""),
      "maple",
      1.1,
      'opacity="0.7"',
    ),
    path(cutRect(cx - 8, 146, 16, 8, r, 0.3), "granite-dark"),
  ].join("");
  const arm = (dx: number, dy: number, letter: string, seed: number) => {
    const ar = rng(seed);
    const end: Pt = [cx + dx, armsY + dy];
    return (
      pencil(
        line([cx, armsY], end, ar, { roughness: 0.3, double: false }),
        "ink",
        2.2,
      ) +
      `<text class="cl-print f-ink" x="${n(end[0] + Math.sign(dx) * 7)}" y="${n(end[1] + (dy > 0 ? 12 : dy < 0 ? -3 : 4.5))}" font-size="13" font-weight="800" text-anchor="middle">${letter}</text>`
    );
  };
  const mast = [
    pencil(`M${cx} 150L${cx} ${pivot + 10}`, "ink", 3),
    arm(-50, 0, "W", 1711),
    arm(50, 0, "E", 1712),
    arm(-12, -14, "N", 1713),
    arm(14, 18, "S", 1714),
    `<circle class="f-ink" cx="${cx}" cy="${armsY}" r="3"/>`,
    `<circle class="f-sun" cx="${cx}" cy="${pivot + 12}" r="4"/>`,
  ].join("");
  const vr = rng(1720);
  const rooster: Pt[] = [
    [-30, -28],
    [-22, -36],
    [-14, -27],
    [-10, -15],
    [4, -13],
    [10, -27],
    [14, -37],
    [19, -41],
    [23, -38],
    [27, -40],
    [28, -34],
    [34, -31],
    [27, -28],
    [26, -22],
    [22, -20],
    [20, -8],
    [12, 0],
    [-6, 2],
    [-18, -4],
    [-30, -13],
    [-35, -22],
  ];
  const vane = [
    pencil(`M-46 10L42 10`, "ink", 2.4),
    path(
      cut(
        [
          [40, 4],
          [54, 10],
          [40, 16],
        ],
        vr,
        0.3,
      ),
      "ink",
    ),
    path(
      cut(
        [
          [-46, 10],
          [-54, 0],
          [-40, 0],
          [-34, 10],
          [-40, 20],
          [-54, 20],
        ],
        vr,
        0.3,
      ),
      "ink",
    ),
    pencil("M2 0L4 10M10 -2L10 10", "ink", 1.8),
    path(cut(rooster, vr, 0.4), "sun"),
    pencil(
      hatch(rooster.slice(15), vr, { angle: 30, gap: 4, roughness: 0.3 }),
      "sun-deep",
      1.4,
    ),
    path(
      cut(
        [
          [17, -39],
          [20, -45],
          [23, -40],
          [26, -45],
          [28, -38],
        ],
        vr,
        0.2,
      ),
      "stamp",
    ),
    `<circle class="f-ink" cx="22" cy="-32" r="1.4"/>`,
    boil(
      (rr) => pencil(curve(wobble(rooster, rr, 0.5), true), "ink", 1.2),
      1721,
    ),
  ].join("");
  const gusts = [
    [
      [8, 30],
      [40, 24],
      [70, 31],
    ],
    [
      [168, 40],
      [198, 34],
      [232, 41],
    ],
    [
      [176, 120],
      [204, 115],
      [234, 121],
    ],
  ]
    .map((pts, i) =>
      pencil(
        curve(pts as unknown as Pt[]),
        "pencil",
        1.5,
        `pathLength="1" style="--gd:${n(i * 0.5)}s"`,
        "cl-gust",
      ),
    )
    .join("");
  const share = (s: { s: string; share: number }) =>
    `${s.s} ${Math.round(s.share)}%`;
  const label = `<g class="cl-pop" style="--pop-d:1.4s"><g transform="rotate(-4 40 140)">${path(cutRect(8, 118, 64, 44, rng(1730), 0.5), "kraft")}<text class="cl-hand f-ink" x="40" y="136" font-size="15" font-weight="700" text-anchor="middle">${share(first)}</text><text class="cl-hand f-ink" x="40" y="154" font-size="15" font-weight="700" text-anchor="middle">${share(second)}</text></g></g>`;
  return {
    w: W,
    h: H,
    layers: [
      { x: 0, y: 0, w: W, h: H, svg: gusts, className: "cl-base" },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: roof,
        enter: { delay: 0, y: 30, kind: "grow" },
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: `<style>@keyframes cl-vane{${keys.join("")}}</style>${mast}<g transform="translate(${cx} ${pivot})"><g class="cl-vane" style="--vane:${fixed(totalTime, 1)}s;transform:scaleX(${fixed(facing(first.a), 2)})">${vane}</g></g>`,
        enter: { delay: 0.25, y: -20, kind: "drop" },
      },
      { x: 0, y: 0, w: W, h: H, svg: label, className: "cl-flat" },
    ],
  };
}
