import type { ReportDay } from "../../reportChart";
import { cut, cutRect, torn, tornRect } from "../paper";
import { boil, path, pencil, type Scene } from "../scene";
import {
  arcPoints,
  f1,
  leafShape,
  skyGlyph,
  stevensonScreen,
  toF,
  toFDelta,
  toIn,
  unitText,
} from "../shapes";
import {
  between,
  curve,
  hatch,
  line,
  n,
  outline,
  type Pt,
  type Rnd,
  ring,
  rng,
} from "../sketch";

function groundStrip(
  w: number,
  y: number,
  h: number,
  seed: number,
  tone = "grass",
): string {
  const r = rng(seed);
  return path(
    tornRect(-6, y, w + 12, h + 10, r, {
      amp: 2.2,
      step: 4,
      torn: [true, false, false, false],
    }),
    tone,
  );
}

// Two folding rulers that unfold to the two normals.
export function rulersScene(n91: number, n96: number): Scene {
  const W = 260;
  const H = 210;
  const base = 172;
  const L = 50;
  const per = (3 * L) / 15;
  const y = (t: number) => base - t * per;
  const ruler = (cx: number, seed: number, delay: number) => {
    const r = rng(seed);
    const seg = (k: number) => {
      const top = -(k + 1) * L;
      const ticks: string[] = [];
      for (let t = 0; t <= 15; t++) {
        const ty = y(t) - base;
        if (ty < top - 0.1 || ty > top + L + 0.1) continue;
        const long = t % 2 === 0;
        ticks.push(
          `M${n(cx - 7.5)} ${n(ty)}L${n(cx - 7.5 + (long ? 7 : 4))} ${n(ty)}`,
        );
      }
      const labels = [2, 4, 6, 8, 10, 12, 14]
        .filter((t) => y(t) - base > top + 3 && y(t) - base < top + L - 2)
        .map(
          (t) =>
            `<text class="cl-print f-ink" x="${n(cx + 5.5)}" y="${n(y(t) - base + 2.6)}" font-size="7" text-anchor="end">${t}</text>`,
        )
        .join("");
      return [
        path(cutRect(cx - 7.5, top + 0.6, 15, L - 1.2, r, 0.4), "birch"),
        pencil(
          line([cx + 6, top + 2], [cx + 6, top + L - 2], r, {
            roughness: 0.3,
            double: false,
          }),
          "sun-deep",
          1.4,
        ),
        pencil(ticks.join(""), "ink", 0.9),
        labels,
        `<circle class="f-ink" cx="${cx}" cy="${n(top + L)}" r="1.5"/>`,
      ].join("");
    };
    const unfold = (k: number, from: number, inner: string) =>
      `<g class="cl-unfold" style="transform-origin:${cx}px ${-k * L}px;--from:${from}deg;--ud:${n(delay + k * 0.38)}s">${seg(k)}${inner}</g>`;
    return `<g transform="translate(0 ${base})">${seg(0)}${unfold(1, 180, unfold(2, -180, ""))}</g>`;
  };
  const flag = (
    cx: number,
    t: number,
    side: 1 | -1,
    metric: string,
    imperial: string,
    seed: number,
    delay: number,
  ) => {
    const r = rng(seed);
    const ty = y(t);
    const x0 = cx + side * 8;
    const pts: Pt[] = [
      [x0, ty],
      [x0 + side * 12, ty - 9],
      [x0 + side * 58, ty - 9],
      [x0 + side * 58, ty + 9],
      [x0 + side * 12, ty + 9],
    ];
    const tx = side === 1 ? x0 + 16 : x0 - 54;
    return `<g class="cl-pop" style="--pop-d:${delay}s">${path(cut(pts, r, 0.5), "paper")}${pencil(outline(pts, r, { roughness: 0.4, double: false }), "ink", 1)}${unitText(tx, ty + 5.5, metric, imperial, 16, 'font-weight="700"')}</g>`;
  };
  const r = rng(201);
  const svg = [
    groundStrip(W, base - 1, 30, 202, "kraft"),
    pencil(
      hatch(
        [
          [0, base + 4],
          [W, base + 4],
          [W, H],
          [0, H],
        ],
        r,
        { angle: 20, gap: 6 },
      ),
      "kraft-dark",
      1.2,
      'opacity="0.6"',
    ),
    ruler(78, 203, 0.3),
    ruler(178, 204, 0.55),
    `<g class="cl-pop" style="--pop-d:2.2s">${pencil(`M${n(78)} ${n(y(n91))}L${n(178)} ${n(y(n91))}`, "stamp", 1.4, 'stroke-dasharray="3 3"')}</g>`,
    flag(78, n91, -1, `${f1(n91)}°`, `${f1(toF(n91))}°`, 205, 1.7),
    flag(178, n96, 1, `${f1(n96, 2)}°`, `${f1(toF(n96), 2)}°`, 206, 1.95),
    `<g class="cl-pop" style="--pop-d:2.4s">${unitText(108, y(n96) - 9, `+${f1(n96 - n91, 2)}°`, `+${f1(toFDelta(n96 - n91), 2)}°`, 15, 'font-weight="700" transform="rotate(-6 108 0)"', "stamp")}</g>`,
    `<text class="cl-hand f-ink" x="78" y="${base + 22}" font-size="17" text-anchor="middle" font-weight="600">1991–2020</text>`,
    `<text class="cl-hand f-ink" x="178" y="${base + 22}" font-size="17" text-anchor="middle" font-weight="600">1996–2025</text>`,
  ].join("");
  return {
    w: W,
    h: H,
    layers: [
      { x: 0, y: 0, w: W, h: H, svg, enter: { delay: 0, y: 6, kind: "fade" } },
    ],
  };
}

// A tear-off desk calendar that flips through the month, one page a day.
export function calendarScene(days: ReportDay[]): Scene {
  const W = 180;
  const H = 222;
  const r = rng(301);
  const board = [
    path(cutRect(12, 14, 156, 200, r, 0.8), "kraft"),
    pencil(
      hatch(
        [
          [14, 16],
          [166, 16],
          [166, 212],
          [14, 212],
        ],
        r,
        { angle: 55, gap: 7, roughness: 0.5 },
      ),
      "kraft-dark",
      1.2,
      'opacity="0.45"',
    ),
    `<circle class="f-ink" cx="90" cy="24" r="3.5" opacity="0.8"/>`,
    pencil(`M30 199L152 199M29 202L153 202`, "kraft-dark", 1.2),
  ].join("");
  const pageShape = cutRect(24, 42, 132, 154, rng(302), 0.5);
  const header = cutRect(24, 60, 132, 24, rng(303), 0.4);
  const holes = `<circle class="f-kraft" cx="62" cy="50" r="3.2"/><circle class="f-kraft" cx="118" cy="50" r="3.2"/>`;
  const pages = [...days]
    .map((d, i) => ({ d, i }))
    .reverse()
    .map(({ d, i }) => {
      const day = Number(d.date.slice(8));
      const last = i === days.length - 1;
      const dir = i % 2 ? 1 : -1;
      const attrs = last
        ? ""
        : ` class="cl-tear" style="--t:${n(0.5 + i * 0.15)}s;--tx:${dir * 22}px;--tr:${dir * 24}deg"`;
      return `<g${attrs}>${path(pageShape, "paper")}${holes}${path(header, "stamp")}<text class="cl-hand f-white" x="90" y="78" font-size="17" font-weight="700" text-anchor="middle" letter-spacing="1.5">SEPTEMBER</text><text class="cl-hand f-ink" x="90" y="150" font-size="64" font-weight="700" text-anchor="middle">${day}</text>${skyGlyph(d.sky ?? "sun", 90, 176, 1.15, 310 + i)}</g>`;
    })
    .join("");
  const rings = [62, 118]
    .map((x) =>
      pencil(
        `M${x} 51C${x - 6} 51 ${x - 6} 33 ${x} 33C${x + 6} 33 ${x + 6} 51 ${x} 51`,
        "ink",
        2.4,
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
        svg: board,
        enter: { delay: 0, y: -30, rotate: -6, kind: "drop" },
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: pages + rings,
        className: "cl-flat",
        enter: { delay: 0.25, y: -30, rotate: -6, kind: "drop" },
      },
    ],
  };
}

// A figure leaning into the wind, scarf streaming, with the feels-like value.
export function feelsScene(feels: number): Scene {
  const W = 260;
  const H = 200;
  const scarfFrames = [0, 1, 2]
    .map((f) => {
      const r = rng(410 + f);
      const pts: Pt[] = Array.from({ length: 7 }, (_, i) => {
        const t = i / 6;
        return [
          92 + t * 62,
          64 - t * 6 + Math.sin(t * 5 + f * 2.1) * (3 + t * 7),
        ] as Pt;
      });
      const top = pts.map(([x, yy], i) => [x, yy - 4.5 + i * 0.25] as Pt);
      const bottom = pts
        .map(([x, yy], i) => [x, yy + 4.5 - i * 0.25] as Pt)
        .reverse();
      const ribbon = [...top, ...bottom];
      const stripes = pts
        .slice(1, -1)
        .map(([x, yy]) => `M${n(x)} ${n(yy - 4)}L${n(x + 2)} ${n(yy + 4)}`)
        .join("");
      const tail = pts.at(-1) as Pt;
      const fringe = [-3, 0, 3]
        .map(
          (dy) =>
            `M${n(tail[0])} ${n(tail[1] + dy)}l${n(between(r, 5, 8))} ${n(dy * 0.8 + between(r, -1, 1))}`,
        )
        .join("");
      return `<g class="cl-cy cl-cy${f}">${path(cut(ribbon, r, 0.4), "scarf")}${pencil(stripes, "white", 2.2)}${pencil(fringe, "scarf", 1.6)}</g>`;
    })
    .join("");
  const r = rng(401);
  const figure = [
    pencil(
      curve([
        [78, 118],
        [72, 140],
        [66, 160],
      ]),
      "ink",
      4,
    ),
    pencil(
      curve([
        [86, 118],
        [92, 140],
        [100, 160],
      ]),
      "ink",
      4,
    ),
    `<ellipse class="f-ink" cx="64" cy="161" rx="6" ry="3"/><ellipse class="f-ink" cx="103" cy="161" rx="6" ry="3"/>`,
    `<g transform="rotate(-9 84 160)">${[
      path(
        cut(
          [
            [68, 64],
            [98, 64],
            [106, 122],
            [60, 122],
          ],
          r,
          0.6,
        ),
        "blue-soft",
      ),
      pencil(
        outline(
          [
            [68, 64],
            [98, 64],
            [106, 122],
            [60, 122],
          ],
          r,
          { roughness: 0.5, double: false },
        ),
        "ink",
        1.3,
      ),
      pencil("M83 70L83 120", "ink", 1, 'opacity="0.5"'),
      `<circle class="f-ink" cx="86" cy="84" r="1.3"/><circle class="f-ink" cx="86" cy="98" r="1.3"/>`,
      `<circle class="f-skin" cx="83" cy="48" r="13"/>`,
      path(
        cut(
          [
            [69, 46],
            ...arcPoints(83, 46, 14.5, Math.PI, 2 * Math.PI, 10),
            [97, 46],
          ],
          r,
          0.4,
        ),
        "stamp",
      ),
      pencil(line([68, 45], [98, 45], r, { double: false }), "white", 3),
      `<circle class="f-white" cx="${n(83)}" cy="29" r="4"/>`,
      pencil("M74 50l5 2.5l-5 2.5M93 50l-5 2.5l5 2.5", "ink", 1.8),
      pencil(
        curve([
          [79, 58],
          [83, 60],
          [87, 58],
        ]),
        "ink",
        1.6,
      ),
      `<ellipse class="f-orange-soft" cx="73" cy="57" rx="3" ry="2" opacity="0.8"/><ellipse class="f-orange-soft" cx="93" cy="57" rx="3" ry="2" opacity="0.8"/>`,
      pencil(
        curve([
          [70, 70],
          [56, 58],
          [64, 38],
        ]),
        "blue-soft",
        6,
      ),
      `<circle class="f-skin" cx="65" cy="37" r="4"/>`,
      pencil(
        curve([
          [96, 72],
          [106, 92],
          [100, 104],
        ]),
        "blue-soft",
        6,
      ),
      path(
        cut(
          [
            [72, 60],
            [94, 60],
            [96, 68],
            [70, 68],
          ],
          r,
          0.4,
        ),
        "scarf",
      ),
      scarfFrames,
    ].join("")}</g>`,
  ].join("");
  const gusts = [
    [
      [8, 54],
      [40, 46],
      [70, 54],
      [96, 44],
    ],
    [
      [2, 92],
      [36, 86],
      [60, 94],
      [92, 84],
    ],
    [
      [14, 128],
      [44, 122],
      [66, 130],
    ],
    [
      [150, 30],
      [186, 22],
      [220, 30],
      [252, 22],
    ],
    [
      [170, 112],
      [206, 104],
      [244, 112],
    ],
  ]
    .map((pts, i) =>
      pencil(
        curve(pts as unknown as Pt[]),
        "pencil",
        1.6,
        `pathLength="1" style="--gd:${n(i * 0.37)}s"`,
        "cl-gust",
      ),
    )
    .join("");
  const label = `${unitText(160, 150, `feels ${f1(feels)}°`, `feels ${f1(toF(feels))}°`, 24, 'font-weight="700" transform="rotate(-4 160 150)"')}`;
  return {
    w: W,
    h: H,
    layers: [
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: groundStrip(W, 166, 34, 402) + gusts,
        className: "cl-base",
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: figure,
        enter: { delay: 0.1, x: -20, rotate: -8, kind: "place" },
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: `<g class="cl-write" style="--write:0.9s">${label}</g>`,
        className: "cl-flat",
        enter: { delay: 0.5, kind: "fade" },
      },
      {
        x: 0,
        y: 140,
        w: 20,
        h: 20,
        svg: leafShape(10, 10, 6.5, 30, "birch", rng(403)),
        className: "cl-tumble",
        vars: { delay: "1.2s" },
      },
    ],
  };
}

// A jar of rain that fills day by day to the month's total, past the normal.
export function rainJarScene(
  days: ReportDay[],
  normal: number,
  pct: number,
): Scene {
  const W = 250;
  const H = 260;
  const bottom = 226;
  const perMm = 1.35;
  const y = (mm: number) => bottom - mm * perMm;
  const total = days.at(-1)?.precip_cum ?? 0;
  const top = y(total);
  const r = rng(501);
  const jar: Pt[] = [
    [86, 70],
    [80, 84],
    [76, 100],
    [74, 118],
    [74, 218],
    [78, 228],
    [88, 234],
    [180, 234],
    [190, 228],
    [194, 218],
    [194, 118],
    [192, 100],
    [188, 84],
    [182, 70],
  ];
  const inside: Pt[] = [
    [88, 74],
    [82, 92],
    [79, 112],
    [79, 216],
    [84, 225],
    [92, 229],
    [176, 229],
    [184, 225],
    [189, 216],
    [189, 112],
    [186, 92],
    [180, 74],
  ];
  const rise = bottom + 6 - top;
  const keyframes = days
    .map(
      (d, i) =>
        `${n(((i + 1) / days.length) * 100)}%{transform:translateY(${n(rise * (1 - d.precip_cum / total))}px)}`,
    )
    .join("");
  const wave = (yy: number) => {
    let d = `M40 ${n(yy)}`;
    for (let x = 40; x < 240; x += 20)
      d += `Q${x + 5} ${n(yy - 3.5)} ${x + 10} ${n(yy)}T${x + 20} ${n(yy)}`;
    return `${d}L240 ${bottom + 30}L40 ${bottom + 30}Z`;
  };
  const water = `<clipPath id="cl-jar-inside"><path d="${curve(inside, true)}"/></clipPath><g clip-path="url(#cl-jar-inside)"><g class="cl-rise" style="--rise:${n(rise)}px"><g class="cl-wave">${path(wave(top), "water")}${path(wave(top + 8), "water-deep", undefined, 'opacity="0.35"')}</g></g></g>`;
  const scale = (marks: number[], toMm: (v: number) => number, unit: string) =>
    pencil(
      marks.map((v) => `M194 ${n(y(toMm(v)))}L202 ${n(y(toMm(v)))}`).join(""),
      "ink",
      1.2,
    ) +
    marks
      .map(
        (v) =>
          `<text class="cl-hand f-ink" x="206" y="${n(y(toMm(v)) + 5)}" font-size="14">${v}</text>`,
      )
      .join("") +
    `<text class="cl-hand f-pencil" x="206" y="${n(y(toMm(marks.at(-1) as number)) - 12)}" font-size="13">${unit}</text>`;
  const ticks = `<g class="u-metric">${scale([20, 40, 60, 80, 100], (v) => v, "mm")}</g><g class="u-imperial">${scale([1, 2, 3], (v) => v * 25.4, "in")}</g>`;
  const rim: Pt[] = [
    [80, 58],
    [188, 58],
    [190, 72],
    [78, 72],
  ];
  const glass = [
    path(curve(jar, true), "glass", undefined, 'opacity="0.6"'),
    water,
    pencil(
      curve([
        [86, 116],
        [84, 160],
        [86, 212],
      ]),
      "white",
      4,
      'opacity="0.8"',
    ),
    pencil(
      curve([
        [94, 106],
        [92, 122],
      ]),
      "white",
      3,
      'opacity="0.7"',
    ),
    pencil(
      `M81 ${n(y(normal))}L187 ${n(y(normal))}`,
      "slate",
      1.8,
      'stroke-dasharray="5 4"',
    ),
    path(
      tornRect(104, 206, 62, 16, r, {
        amp: 1.2,
        torn: [false, true, false, true],
      }),
      "tape-cream",
    ),
    `<text class="cl-hand f-ink" x="135" y="218" font-size="11" text-anchor="middle" font-weight="700">KAISANIEMI</text>`,
    boil(
      (rr) =>
        pencil(
          curve(
            jar.map(
              ([x, yy]) =>
                [x + between(rr, -0.7, 0.7), yy + between(rr, -0.7, 0.7)] as Pt,
            ),
            true,
          ),
          "ink",
          1.6,
        ),
      502,
    ),
    path(cut(rim, r, 0.4), "glass"),
    pencil(
      curve([
        [86, 62],
        [134, 60],
        [182, 62],
      ]),
      "white",
      2.4,
      'opacity="0.9"',
    ),
    boil(
      (rr) =>
        pencil(outline(rim, rr, { roughness: 0.4, double: false }), "ink", 1.5),
      503,
    ),
  ].join("");
  const normalLabel = `<g class="cl-pop" style="--pop-d:0.6s">${pencil(`M58 ${n(y(normal))}L72 ${n(y(normal))}`, "slate", 1.4)}${unitText(4, y(normal) - 3, "normal", "normal", 15, 'font-weight="600"', "slate")}${unitText(4, y(normal) + 13, `${f1(normal)} mm`, `${f1(toIn(normal), 2)} in`, 14, "", "slate")}</g>`;
  const finalLabel = `<g class="cl-pop" style="--pop-d:4.9s">${pencil(
    curve([
      [52, top + 8],
      [62, top + 2],
      [74, top + 2],
    ]),
    "ink",
    1.5,
  )}${pencil(`M68 ${n(top - 2)}L74 ${n(top + 2)}L68 ${n(top + 6)}`, "ink", 1.5)}${unitText(2, top + 4, `${f1(total)} mm`, `${f1(toIn(total), 2)} in`, 18, 'font-weight="700"')}</g>`;
  const drops = [0, 1, 2]
    .map((i) => {
      const x = 116 + i * 18;
      return `<g class="cl-drop-fall" style="--dd:${n(i * 0.27)}s"><path class="f-rain" d="M${x} 0C${x - 4} 7 ${x - 4} 11 ${x} 11C${x + 4} 11 ${x + 4} 7 ${x} 0Z"/></g>`;
    })
    .join("");
  const stamp = `<g transform="rotate(-11 134 168)">${path(cutRect(90, 148, 88, 40, rng(505), 0.8), "paper", undefined, 'fill-opacity="0.35"')}${pencil(
    outline(
      [
        [90, 148],
        [178, 148],
        [178, 188],
        [90, 188],
      ],
      rng(506),
      { roughness: 0.6 },
    ),
    "stamp",
    2.6,
  )}<text class="cl-print f-stamp" x="134" y="179" font-size="27" font-weight="800" text-anchor="middle" letter-spacing="0.5">${pct}%</text></g>`;
  return {
    w: W,
    h: H,
    layers: [
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: `<style>@keyframes cl-jar-rise{0%{transform:translateY(${n(rise)}px)}${keyframes}}</style>${glass}${ticks}`,
        enter: { delay: 0, y: 20, kind: "drop" },
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: normalLabel + finalLabel,
        className: "cl-flat",
      },
      { x: 0, y: 8, w: W, h: 80, svg: drops, className: "cl-flat cl-drops" },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: stamp,
        className: "cl-stamp cl-flat",
        vars: { sd: "5.1s", so: "54% 65%" },
      },
    ],
  };
}

// A windsock streaming in the breeze.
export function windsockScene(): Scene {
  const W = 240;
  const H = 180;
  const frames = [0, 1, 2]
    .map((f) => {
      const r = rng(610 + f);
      const len = 128;
      const center = (t: number): Pt => [
        44 + t * len,
        40 + t * 8 + Math.sin(t * 4 + f * 2.1) * t * 9,
      ];
      const half = (t: number) => 13 - t * 6;
      const stripes = [0, 1, 2, 3, 4]
        .map((k) => {
          const pts: Pt[] = [];
          for (let s = 0; s <= 4; s++) {
            const t = (k + s / 4) / 5;
            const [cx, cy] = center(t);
            pts.push([cx, cy - half(t)]);
          }
          for (let s = 4; s >= 0; s--) {
            const t = (k + s / 4) / 5;
            const [cx, cy] = center(t);
            pts.push([cx, cy + half(t)]);
          }
          return path(cut(pts, r, 0.4), k % 2 ? "white" : "orange");
        })
        .join("");
      const outlinePts: Pt[] = [];
      for (let s = 0; s <= 10; s++)
        outlinePts.push([center(s / 10)[0], center(s / 10)[1] - half(s / 10)]);
      for (let s = 10; s >= 0; s--)
        outlinePts.push([center(s / 10)[0], center(s / 10)[1] + half(s / 10)]);
      return `<g class="cl-cy cl-cy${f}">${stripes}${pencil(curve(outlinePts, true), "ink", 1.3)}</g>`;
    })
    .join("");
  const r = rng(601);
  const pole = [
    pencil(
      curve([
        [42, 168],
        [43, 100],
        [42, 26],
      ]),
      "ink",
      3,
    ),
    `<circle class="f-ink" cx="42" cy="25" r="3"/>`,
    path(cutRect(36, 34, 12, 12, r, 0.3), "granite"),
  ].join("");
  const gusts = [
    [
      [96, 86],
      [130, 80],
      [160, 88],
      [196, 80],
    ],
    [
      [120, 116],
      [156, 110],
      [190, 118],
      [226, 110],
    ],
    [
      [70, 140],
      [104, 134],
      [136, 140],
    ],
  ]
    .map((pts, i) =>
      pencil(
        curve(pts as unknown as Pt[]),
        "pencil",
        1.5,
        `pathLength="1" style="--gd:${n(i * 0.45)}s"`,
        "cl-gust",
      ),
    )
    .join("");
  const cloudPts = ring(196, 30, 26, 11, 20).map(
    ([x, yy]) => [x + between(r, -1.5, 1.5), yy + between(r, -1.5, 1.5)] as Pt,
  );
  return {
    w: W,
    h: H,
    layers: [
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: groundStrip(W, 162, 30, 602) + gusts,
        className: "cl-base",
      },
      {
        x: 150,
        y: 6,
        w: 90,
        h: 50,
        svg: `<g transform="translate(-150 -6)">${path(torn(cloudPts, r, { amp: 1.6 }), "cloud")}${pencil(hatch(cloudPts.slice(0, 10), r, { gap: 5 }), "cloud-shade", 1.6)}</g>`,
        loop: "cl-drift",
        vars: { drift: "9s", dx: "-30%" },
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: pole + frames,
        enter: { delay: 0.1, y: 30, kind: "grow" },
      },
    ],
  };
}

// A cold radiator having a nap.
export function radiatorScene(): Scene {
  const W = 240;
  const H = 180;
  const r = rng(701);
  const fins = Array.from({ length: 7 }, (_, i) => {
    const x = 58 + i * 18;
    const fin: Pt[] = [
      [x, 66],
      [x + 4, 60],
      [x + 11, 60],
      [x + 15, 66],
      [x + 15, 142],
      [x + 11, 148],
      [x + 4, 148],
      [x, 142],
    ];
    return (
      path(cut(fin, r, 0.4), "radiator") +
      pencil(
        line([x + 11.5, 66], [x + 11.5, 142], r, {
          roughness: 0.3,
          double: false,
        }),
        "cloud-shade",
        3.4,
      ) +
      pencil(
        outline(fin, r, { roughness: 0.3, double: false }),
        "pencil",
        1,
        'opacity="0.8"',
      )
    );
  }).join("");
  const body = [
    pencil(`M60 152L60 162M178 152L178 162`, "ink", 3),
    path(cutRect(54, 76, 132, 8, r, 0.3), "cloud-shade"),
    path(cutRect(54, 130, 132, 8, r, 0.3), "cloud-shade"),
    fins,
    boil(
      (rr) =>
        pencil(
          outline(
            [
              [56, 62],
              [186, 62],
              [186, 148],
              [56, 148],
            ],
            rr,
            { roughness: 0.6, double: false },
          ),
          "ink",
          1.3,
          'opacity="0.7"',
        ),
      702,
    ),
    pencil(
      curve([
        [54, 132],
        [40, 132],
        [36, 144],
      ]),
      "granite-dark",
      5,
    ),
    `<circle class="f-stamp" cx="38" cy="122" r="7"/>`,
    pencil("M38 122L38 130", "ink", 2),
    boil(
      () =>
        [
          pencil(
            curve([
              [96, 100],
              [102, 106],
              [108, 100],
            ]),
            "ink",
            2.4,
          ),
          pencil(
            curve([
              [130, 100],
              [136, 106],
              [142, 100],
            ]),
            "ink",
            2.4,
          ),
          pencil(
            curve([
              [112, 116],
              [119, 120],
              [126, 116],
            ]),
            "ink",
            2,
          ),
        ].join(""),
      703,
    ),
    `<ellipse class="f-orange-soft" cx="94" cy="112" rx="5" ry="3" opacity="0.7"/><ellipse class="f-orange-soft" cx="144" cy="112" rx="5" ry="3" opacity="0.7"/>`,
  ].join("");
  const zs = [0, 1, 2]
    .map(
      (i) =>
        `<text class="cl-hand f-blue cl-z" x="${190 + i * 12}" y="${58 - i * 14}" font-size="${18 + i * 6}" font-weight="700" style="--zd:${n(i * 0.8)}s">z</text>`,
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
        svg:
          path(
            tornRect(-6, 160, W + 12, 30, r, {
              amp: 2,
              torn: [true, false, false, false],
            }),
            "kraft",
          ) + leafShape(206, 158, 7, 80, "birch", r),
        className: "cl-base",
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: body,
        enter: { delay: 0.1, y: 20, kind: "drop" },
      },
      { x: 0, y: 0, w: W, h: H, svg: zs, className: "cl-flat" },
    ],
  };
}

// The Stevenson screen gets the open-data stamp.
export function stampScene(): Scene {
  const W = 260;
  const H = 180;
  const r = rng(801);
  const mark = (rr: Rnd) => {
    const specks = Array.from(
      { length: 26 },
      () =>
        `<circle class="f-paper" cx="${n(between(rr, 136, 234))}" cy="${n(between(rr, 72, 144))}" r="${n(between(rr, 0.6, 1.8))}"/>`,
    ).join("");
    return `<g transform="rotate(-9 185 108)">${pencil(
      outline(
        [
          [136, 72],
          [234, 72],
          [234, 144],
          [136, 144],
        ],
        rr,
        { roughness: 0.8 },
      ),
      "stamp",
      3,
    )}${pencil(
      outline(
        [
          [142, 78],
          [228, 78],
          [228, 138],
          [142, 138],
        ],
        rr,
        { roughness: 0.5, double: false },
      ),
      "stamp",
      1.4,
    )}<text class="cl-print f-stamp" x="185" y="106" font-size="26" font-weight="800" text-anchor="middle" letter-spacing="2">FMI</text><text class="cl-print f-stamp" x="185" y="120" font-size="9" font-weight="700" text-anchor="middle" letter-spacing="1.4">OPEN DATA</text><text class="cl-print f-stamp" x="185" y="131" font-size="8" font-weight="700" text-anchor="middle" letter-spacing="1">CC BY 4.0</text>${specks}</g>`;
  };
  const tool = [
    path(
      cut(
        [
          [166, 2],
          [204, 2],
          [208, 20],
          [162, 20],
        ],
        r,
        0.4,
      ),
      "kraft-dark",
    ),
    `<ellipse class="f-kraft" cx="185" cy="2" rx="16" ry="7"/>`,
    path(cutRect(150, 20, 70, 14, r, 0.4), "kraft"),
    path(cutRect(152, 34, 66, 8, r, 0.3), "stamp"),
    pencil(
      outline(
        [
          [150, 20],
          [220, 20],
          [220, 34],
          [150, 34],
        ],
        r,
        { roughness: 0.4, double: false },
      ),
      "ink",
      1.2,
    ),
  ].join("");
  return {
    w: W,
    h: H,
    layers: [
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: path(
          tornRect(-6, 152, W + 12, 40, r, {
            amp: 2,
            torn: [true, false, false, false],
          }),
          "grass",
        ),
        className: "cl-base",
      },
      {
        x: 4,
        y: 20,
        w: 110,
        h: 140,
        svg: stevensonScreen(0, 0, 1.55, 802),
        enter: { delay: 0, y: -40, kind: "drop" },
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: mark(r),
        className: "cl-stamp-mark cl-flat",
      },
      { x: 0, y: 0, w: W, h: H, svg: tool, className: "cl-stamp-tool" },
    ],
  };
}

// A Campbell–Stokes sunshine recorder: as a sun crosses overhead, the glass
// ball scorches the card hour by hour, each mark as strong as that hour's
// average share of sunshine through the month.
export function recorderScene(profile: number[], total: number): Scene {
  const W = 240;
  const H = 200;
  const r = rng(901);
  const cx = 120;
  const cy = 84;
  const deg = (a: number) => (a * Math.PI) / 180;
  const at = (radius: number, a: number): Pt => [
    cx + radius * Math.cos(deg(a)),
    cy + radius * Math.sin(deg(a)),
  ];
  const arc = (radius: number, from: number, to: number, count = 24): Pt[] =>
    Array.from({ length: count + 1 }, (_, i) =>
      at(radius, from + ((to - from) * i) / count),
    );
  const hourAngle = (h: number) => 160 - ((h - 6) / 14) * 140;
  const band = (r0: number, r1: number) => [
    ...arc(r1, 165, 15),
    ...arc(r0, 15, 165),
  ];
  const ticks = Array.from({ length: 15 }, (_, i) => {
    const a = hourAngle(6 + i);
    const [x0, y0] = at(i % 2 ? 46 : 44.5, a);
    const [x1, y1] = at(48, a);
    return `M${n(x0)} ${n(y0)}L${n(x1)} ${n(y1)}`;
  }).join("");
  const sweep = 4.2;
  const trace = Array.from({ length: 14 }, (_, i) => {
    const f = profile[6 + i] ?? 0;
    if (f < 0.02) return "";
    const pts = arc(41, hourAngle(6 + i), hourAngle(7 + i), 4);
    const style = `--draw:${n(sweep / 14)}s;--draw-d:${n(0.5 + (i / 14) * sweep)}s`;
    return `<path class="s-ink cl-draw" d="${curve(pts)}" fill="none" stroke-width="${n(1.6 + 9 * f)}" stroke-linecap="butt" opacity="${n(Math.min(0.95, 0.4 + 1.1 * f))}" pathLength="1" style="${style}"/>`;
  }).join("");
  const sunAt = (a: number) => at(76, a);
  const [tx, ty] = sunAt(270);
  const frames = [200, 218, 236, 254, 270, 286, 304, 322, 340]
    .map((a, i, all) => {
      const [x, y] = sunAt(a);
      return `${n((i / (all.length - 1)) * 100)}%{transform:translate(${n(x - tx)}px,${n(y - ty)}px)}`;
    })
    .join("");
  const sun = `<g class="cl-sun-path">${skyGlyph("sun", tx, ty, 1.25, 902)}</g>`;
  const glass = [
    `<circle class="f-glass" cx="${cx}" cy="${cy}" r="31" opacity="0.8"/>`,
    pencil(curve(arc(24, 200, 250, 6)), "white", 3, 'opacity="0.9"'),
    pencil(curve(arc(18, 290, 320, 4)), "white", 2, 'opacity="0.6"'),
    `<circle class="f-white" cx="${cx - 12}" cy="${cy - 14}" r="3.4" opacity="0.9"/>`,
    boil(
      (rr) =>
        pencil(
          curve(
            arc(31, 0, 362, 18).map(
              ([x, y]) =>
                [x + between(rr, -0.6, 0.6), y + between(rr, -0.6, 0.6)] as Pt,
            ),
          ),
          "ink",
          1.4,
        ),
      903,
    ),
  ].join("");
  const holder = [
    path(cut(band(48, 55), r, 0.3), "granite-dark"),
    path(cut(band(34, 48), r, 0.2), "paper"),
    pencil(ticks, "pencil", 1),
    trace,
    boil(
      (rr) =>
        pencil(
          curve(
            arc(55, 165, 15, 12).map(
              ([x, y]) =>
                [x + between(rr, -0.6, 0.6), y + between(rr, -0.6, 0.6)] as Pt,
            ),
          ),
          "ink",
          1.3,
        ),
      904,
    ),
  ].join("");
  const pedestal = [
    path(cutRect(100, 136, 40, 44, r, 0.5), "granite"),
    pencil(
      hatch(
        [
          [100, 138],
          [112, 138],
          [112, 178],
          [100, 178],
        ],
        r,
        { angle: 75, gap: 4 },
      ),
      "granite-dark",
      1.4,
    ),
    path(cutRect(92, 128, 56, 10, r, 0.4), "granite-dark"),
    path(cutRect(115, 114, 10, 16, r, 0.3), "granite-dark"),
    boil(
      (rr) =>
        pencil(
          outline(
            [
              [100, 180],
              [100, 138],
              [140, 138],
              [140, 180],
            ],
            rr,
            { roughness: 0.4, double: false, closed: false },
          ),
          "ink",
          1.3,
        ),
      905,
    ),
  ].join("");
  const tag = `<g transform="rotate(-6 196 160)">${path(
    cut(
      [
        [166, 146],
        [226, 146],
        [226, 172],
        [166, 172],
        [160, 159],
      ],
      r,
      0.5,
    ),
    "kraft",
  )}<circle class="f-paper" cx="167" cy="159" r="2"/><text class="cl-hand f-ink" x="198" y="165" font-size="17" font-weight="700" text-anchor="middle">${f1(total)} h</text></g>`;
  return {
    w: W,
    h: H,
    layers: [
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: groundStrip(W, 176, 30, 906, "kraft"),
        className: "cl-base",
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: `<style>@keyframes cl-sun-path{${frames}}</style>${sun}`,
        className: "cl-flat",
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg: pedestal + holder + glass,
        enter: { delay: 0, y: 20, kind: "drop" },
      },
      {
        x: 0,
        y: 0,
        w: W,
        h: H,
        svg:
          pencil(
            curve([
              [150, 158],
              [158, 152],
              [164, 156],
            ]),
            "ink",
            1,
          ) + tag,
        enter: { delay: 4.8, y: -20, rotate: 8, kind: "drop" },
      },
    ],
  };
}
