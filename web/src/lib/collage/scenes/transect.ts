import type { ReportStation } from "../../reportChart";
import { cut, cutCircle, cutRect, tape, torn, tornRect } from "../paper";
import { boil, type Layer, path, pencil, type Scene } from "../scene";
import { arcPoints, f1, grow, stevensonScreen, toF, unitText } from "../shapes";
import {
  between,
  cloudShape,
  cloudTop,
  curve,
  ellipse,
  hatch,
  line,
  n,
  outline,
  type Pt,
  rng,
} from "../sketch";

const W = 960;
const H = 400;
const HORIZON = 262;
const LOW = 10;
const HIGH = 15;
const TUBE_TOP = 112;
const TUBE_BOTTOM = 292;

function tempY(t: number): number {
  return TUBE_BOTTOM - ((t - LOW) / (HIGH - LOW)) * (TUBE_BOTTOM - TUBE_TOP);
}

function sky(): Layer {
  const r = rng(1001);
  const band = tornRect(-10, -10, W + 20, 120, r, {
    amp: 3.5,
    step: 6,
    torn: [false, false, true, false],
  });
  return {
    x: 0,
    y: 0,
    w: W,
    h: H,
    svg: `<rect class="f-sky" width="${W}" height="${H}"/>${path(band, "sky-deep")}${pencil(
      hatch(
        [
          [20, 130],
          [260, 120],
          [240, 200],
          [10, 210],
        ],
        r,
        { angle: -30, gap: 10 },
      ),
      "sky-deep",
      1.5,
      'opacity="0.8"',
    )}`,
    className: "cl-base",
  };
}

function cloudLayer(
  x: number,
  y: number,
  w: number,
  h: number,
  seed: number,
  delay: number,
  drift: string,
  dx: string,
): Layer {
  const r = rng(seed);
  const pts = cloudShape(6, 6, w - 12, h - 12, r, 4);
  const svg = [
    path(torn(grow(pts, 2), r, { amp: 1.8 }), "fiber"),
    path(torn(pts, r, { amp: 1.6 }), "cloud"),
    pencil(
      hatch(
        pts.map(([px, py]) => [px, Math.max(py, h * 0.62)] as Pt),
        r,
        { angle: -28, gap: 5 },
      ),
      "cloud-shade",
      2,
    ),
    boil(
      (rr) =>
        pencil(
          curve(
            cloudTop(pts, 5).map(
              ([px, py]) => [px + 2 + between(rr, -0.7, 0.7), py - 2] as Pt,
            ),
          ),
          "ink",
          1.2,
          'opacity="0.55"',
        ),
      seed,
    ),
  ].join("");
  return {
    x,
    y,
    w,
    h,
    svg,
    enter: { delay, x: -20, kind: "place" },
    loop: "cl-drift",
    vars: { drift, dx },
  };
}

function land(): Layer {
  const r = rng(1011);
  const pts: Pt[] = [[-10, HORIZON + 4]];
  for (let x = 0; x <= 800; x += 20) {
    const hill =
      x > 250 && x < 450 ? Math.sin(((x - 250) / 200) * Math.PI) * 34 : 0;
    const shore = x > 720 ? (x - 720) * 0.45 : 0;
    pts.push([x, HORIZON - hill + shore + between(r, -1.5, 1.5)]);
  }
  pts.push([820, H + 10], [-10, H + 10]);
  const svg = [
    path(
      torn(pts, r, {
        amp: 2.4,
        step: 5,
        torn: pts.map((_, i) => i < pts.length - 2),
      }),
      "grass",
    ),
    pencil(
      hatch(
        [
          [0, HORIZON + 6],
          [720, HORIZON + 6],
          [720, HORIZON + 40],
          [0, HORIZON + 40],
        ],
        r,
        { angle: -60, gap: 8 },
      ),
      "grass-dark",
      1.8,
      'opacity="0.55"',
    ),
  ].join("");
  return {
    x: 0,
    y: 0,
    w: W,
    h: H,
    svg,
    enter: { delay: 0.2, y: 20, kind: "grow" },
  };
}

function sea(): Layer {
  const r = rng(1021);
  const pts: Pt[] = [
    [745, HORIZON + 6],
    [W + 10, HORIZON - 2],
    [W + 10, H + 10],
    [800, H + 10],
  ];
  const waves = Array.from({ length: 9 }, (_, i) => {
    const y = HORIZON + 18 + i * 14;
    const x0 = 770 + (i % 2) * 18 + between(r, -6, 6);
    let d = `M${n(x0)} ${n(y)}`;
    for (let x = x0; x < W - 10; x += 22) d += `q5.5 -4 11 0t11 0`;
    return d;
  }).join("");
  const svg = [
    path(torn(pts, r, { amp: 2, torn: [true, false, false, true] }), "water"),
    `<g class="cl-wave" style="--wl:-22px">${pencil(waves, "water-deep", 1.6, 'opacity="0.7"')}</g>`,
  ].join("");
  return { x: 0, y: 0, w: W, h: H, svg, enter: { delay: 0.25, kind: "fade" } };
}

function airport(): Layer {
  const r = rng(1031);
  const dashes = Array.from(
    { length: 9 },
    (_, i) => `M${30 + i * 22} ${HORIZON + 9}L${42 + i * 22} ${HORIZON + 9}`,
  ).join("");
  const svg = [
    path(
      cut(
        [
          [10, HORIZON + 3],
          [236, HORIZON + 3],
          [240, HORIZON + 15],
          [6, HORIZON + 15],
        ],
        r,
        0.6,
      ),
      "granite-dark",
    ),
    pencil(dashes, "white", 2),
    path(cutRect(186, 196, 12, 64, r, 0.4), "granite"),
    path(
      cut(
        [
          [178, 196],
          [206, 196],
          [210, 182],
          [174, 182],
        ],
        r,
        0.4,
      ),
      "glass",
    ),
    path(cutRect(176, 178, 32, 5, r, 0.3), "granite-dark"),
    boil(
      (rr) =>
        pencil(
          outline(
            [
              [174, 182],
              [210, 182],
              [206, 196],
              [178, 196],
            ],
            rr,
            { roughness: 0.4, double: false },
          ) +
            line([186, 196], [186, 260], rr, { double: false }) +
            line([198, 196], [198, 260], rr, { double: false }),
          "ink",
          1.2,
        ),
      1032,
    ),
  ].join("");
  return {
    x: 0,
    y: 0,
    w: W,
    h: H,
    svg,
    enter: { delay: 0.45, y: 20, kind: "grow" },
  };
}

function kumpula(): Layer {
  const r = rng(1041);
  const svg = [
    path(cutRect(372, 214, 86, 30, r, 0.5), "block-warm"),
    ...[0, 1, 2, 3, 4].map((i) =>
      path(cutRect(380 + i * 15, 222, 8, 12, r, 0.3), "window"),
    ),
    path(cutRect(408, 200, 18, 15, r, 0.3), "granite"),
    path(cutCircle(417, 190, 14, r, 18), "white"),
    boil(
      (rr) =>
        pencil(
          ellipse(417, 190, 14, 14, rr, { double: false, roughness: 0.4 }) +
            outline(
              [
                [372, 214],
                [458, 214],
                [458, 244],
                [372, 244],
              ],
              rr,
              { roughness: 0.4, double: false },
            ),
          "ink",
          1.2,
        ),
      1042,
    ),
    pencil(
      curve([
        [406, 186],
        [417, 182],
        [428, 186],
      ]) +
        curve([
          [404, 194],
          [417, 198],
          [430, 194],
        ]),
      "pencil",
      1,
      'opacity="0.7"',
    ),
  ].join("");
  return {
    x: 0,
    y: 0,
    w: W,
    h: H,
    svg,
    enter: { delay: 0.55, y: 20, kind: "grow" },
  };
}

function city(): Layer {
  const r = rng(1051);
  const cx = 512;
  const dome: Pt[] = [
    [cx - 18, 214],
    ...arcPoints(cx, 214, 18, Math.PI, 2 * Math.PI, 10).map(
      ([x, y]) => [x, y - (214 - y) * 0.5] as Pt,
    ),
    [cx + 18, 214],
  ];
  const svg = [
    path(cutRect(440, 230, 36, 34, r, 0.5), "block"),
    path(cutRect(cx - 34, 214, 68, 50, r, 0.5), "wall"),
    path(
      cut(
        [
          [cx - 24, 230],
          [cx, 216],
          [cx + 24, 230],
        ],
        r,
        0.3,
      ),
      "wall-shade",
    ),
    path(cut(dome, r, 0.4), "copper"),
    pencil(`M${cx} 187L${cx} 177M${cx - 4} 181L${cx + 4} 181`, "ink", 1.3),
    ...[-22, -12, 12, 22].map((dx) =>
      path(cutRect(cx + dx - 2, 236, 4, 22, r, 0.2), "white"),
    ),
    path(cutRect(624, 236, 76, 28, r, 0.5), "granite-dark"),
    path(cutRect(644, 150, 22, 114, r, 0.5), "granite"),
    path(
      cut(
        [
          [644, 150],
          [655, 128],
          [666, 150],
        ],
        r,
        0.3,
      ),
      "copper",
    ),
    `<circle class="f-paper" cx="655" cy="162" r="6"/>`,
    pencil("M655 162L655 158M655 162L658 162", "ink", 1),
    path(cutRect(704, 226, 34, 38, r, 0.5), "block-warm"),
    boil(
      (rr) =>
        pencil(
          outline(dome, rr, { roughness: 0.4, double: false, closed: false }) +
            outline(
              [
                [644, 236],
                [644, 150],
                [666, 150],
                [666, 236],
              ],
              rr,
              { roughness: 0.4, double: false, closed: false },
            ),
          "ink",
          1.2,
        ),
      1052,
    ),
    `<g transform="translate(700 222) scale(0.5)">${stevensonScreen(0, 0, 1, 1053)}</g>`,
  ].join("");
  return {
    x: 0,
    y: 0,
    w: W,
    h: H,
    svg,
    enter: { delay: 0.65, y: 20, kind: "grow" },
  };
}

function harmaja(): Layer[] {
  const r = rng(1061);
  const rock: Pt[] = [
    [868, HORIZON + 4],
    [878, HORIZON - 10],
    [896, HORIZON - 16],
    [924, HORIZON - 14],
    [944, HORIZON - 6],
    [956, HORIZON + 6],
  ];
  const island = [
    path(cut(rock, r, 1), "granite"),
    pencil(hatch(rock, r, { angle: 40, gap: 5 }), "granite-dark", 1.4),
  ].join("");
  const tower = [
    path(
      cut(
        [
          [900, HORIZON - 14],
          [904, 176],
          [918, 176],
          [922, HORIZON - 14],
        ],
        r,
        0.4,
      ),
      "white",
    ),
    path(
      cut(
        [
          [902, 206],
          [920, 206],
          [921, 220],
          [901, 220],
        ],
        r,
        0.3,
      ),
      "stamp",
    ),
    path(cutRect(900, 160, 22, 17, r, 0.3), "glass"),
    path(
      cut(
        [
          [897, 160],
          [911, 146],
          [925, 160],
        ],
        r,
        0.3,
      ),
      "stamp",
    ),
    `<circle class="f-sun cl-glow" cx="911" cy="168" r="10" opacity="0.4"/>`,
    boil(
      (rr) =>
        pencil(
          outline(
            [
              [900, HORIZON - 14],
              [904, 176],
              [918, 176],
              [922, HORIZON - 14],
            ],
            rr,
            { roughness: 0.4, double: false, closed: false },
          ) +
            outline(
              [
                [897, 160],
                [911, 146],
                [925, 160],
              ],
              rr,
              { roughness: 0.4, double: false },
            ),
          "ink",
          1.2,
        ),
      1062,
    ),
  ].join("");
  const beam = (dir: 1 | -1, cls: string) =>
    `<path class="f-sun ${cls}" d="M911 168L${911 + dir * 90} ${150}L${911 + dir * 90} ${186}Z" opacity="0.32"/>`;
  return [
    {
      x: 0,
      y: 0,
      w: W,
      h: H,
      svg: island,
      enter: { delay: 0.75, y: 10, kind: "grow" },
    },
    {
      x: 0,
      y: 0,
      w: W,
      h: H,
      svg: beam(-1, "cl-beam cl-beam-a") + beam(1, "cl-beam cl-beam-b"),
      className: "cl-flat",
    },
    {
      x: 0,
      y: 0,
      w: W,
      h: H,
      svg: tower,
      enter: { delay: 0.85, y: 20, kind: "grow" },
    },
  ];
}

function plane(): Layer {
  const r = rng(1071);
  const body: Pt[] = [
    [4, 14],
    [14, 10],
    [48, 9],
    [58, 12],
    [56, 16],
    [12, 18],
  ];
  const svg = [
    path(cut(body, r, 0.3), "white"),
    path(
      cut(
        [
          [8, 12],
          [4, 2],
          [12, 3],
          [18, 11],
        ],
        r,
        0.3,
      ),
      "blue-soft",
    ),
    path(
      cut(
        [
          [24, 14],
          [34, 14],
          [24, 24],
        ],
        r,
        0.3,
      ),
      "cloud-shade",
    ),
    pencil(outline(body, r, { roughness: 0.3, double: false }), "ink", 1),
    pencil("M24 12L44 12", "blue-soft", 1.6, 'stroke-dasharray="2.5 2.5"'),
  ].join("");
  return { x: 24, y: HORIZON - 20, w: 62, h: 26, svg, className: "cl-plane" };
}

function gull(): Layer {
  const wing = (lift: number) =>
    pencil(
      curve([
        [2, 10 - lift * 0.4],
        [10, 6 - lift],
        [16, 10],
      ]) +
        curve([
          [16, 10],
          [22, 6 - lift],
          [30, 10 - lift * 0.4],
        ]),
      "ink",
      1.8,
    );
  const svg = [6, 1, -4]
    .map((lift, i) => `<g class="cl-cy cl-cy${i}">${wing(lift)}</g>`)
    .join("");
  return { x: 720, y: 120, w: 32, h: 18, svg, className: "cl-gull" };
}

function foreground(): Layer {
  const r = rng(1081);
  const svg = [
    path(
      tornRect(-10, 318, 800, 100, r, {
        amp: 2.6,
        step: 5,
        torn: [true, true, false, false],
      }),
      "grass-dark",
    ),
    pencil(
      hatch(
        [
          [0, 326],
          [780, 326],
          [780, 360],
          [0, 360],
        ],
        r,
        { angle: -55, gap: 9 },
      ),
      "grass",
      1.6,
      'opacity="0.5"',
    ),
  ].join("");
  return {
    x: 0,
    y: 0,
    w: W,
    h: H,
    svg,
    enter: { delay: 0.35, y: 30, kind: "grow" },
  };
}

function thermometer(
  x: number,
  station: ReportStation,
  index: number,
  seed: number,
  floating: boolean,
): Layer[] {
  const r = rng(seed);
  const w = 92;
  const h = 250;
  const cx = 46;
  const local = (yy: number) => yy - (TUBE_TOP - 26);
  const tube: Pt[] = [
    [cx - 8, local(TUBE_TOP - 12)],
    [cx + 8, local(TUBE_TOP - 12)],
    [cx + 8, local(TUBE_BOTTOM + 8)],
    [cx - 8, local(TUBE_BOTTOM + 8)],
  ];
  const bulbY = local(TUBE_BOTTOM + 20);
  const ticks: string[] = [];
  const labels: string[] = [];
  for (let t = LOW; t <= HIGH; t++) {
    const yy = local(tempY(t));
    ticks.push(`M${cx - 8} ${n(yy)}L${cx - 14} ${n(yy)}`);
    labels.push(
      `<text class="cl-print f-pencil u-metric" x="${cx - 16}" y="${n(yy + 3)}" font-size="9" text-anchor="end">${t}</text>`,
    );
  }
  for (let f = 50; f <= 59; f += 2) {
    const yy = local(tempY(((f - 32) * 5) / 9));
    labels.push(
      `<text class="cl-print f-pencil u-imperial" x="${cx - 16}" y="${n(yy + 3)}" font-size="9" text-anchor="end">${f}</text>`,
    );
  }
  for (let t = LOW + 0.5; t < HIGH; t++)
    ticks.push(
      `M${cx - 8} ${n(local(tempY(t)))}L${cx - 11} ${n(local(tempY(t)))}`,
    );
  const top = local(tempY(station.temp_avg));
  const feels =
    station.feels_avg != null ? local(tempY(station.feels_avg)) : null;
  const mercury = `<g class="cl-mercury" style="transform-origin:${cx}px ${n(bulbY)}px;--md:${n(0.9 + index * 0.35)}s">${path(cutRect(cx - 3.6, top, 7.2, bulbY - top, r, 0.2), "orange")}</g>`;
  const feelsTab =
    feels == null
      ? ""
      : `<g class="cl-pop" style="--pop-d:${n(2.3 + index * 0.35)}s">${path(
          cut(
            [
              [cx + 9, feels],
              [cx + 17, feels - 7],
              [cx + 44, feels - 7],
              [cx + 44, feels + 7],
              [cx + 17, feels + 7],
            ],
            r,
            0.4,
          ),
          "blue-soft",
        )}<text class="cl-hand f-ink" x="${cx + 19}" y="${n(feels + 4.5)}" font-size="13" font-weight="700">feels</text></g>`;
  const glass = [
    path(cut(tube, r, 0.4), "white"),
    `<circle class="f-white" cx="${cx}" cy="${n(bulbY)}" r="15"/>`,
    `<circle class="f-orange" cx="${cx}" cy="${n(bulbY)}" r="10"/>`,
    mercury,
    pencil(ticks.join(""), "ink", 1),
    labels.join(""),
    pencil(
      line([cx + 4, local(TUBE_TOP - 6)], [cx + 4, local(TUBE_BOTTOM)], r, {
        double: false,
      }),
      "glass",
      2.4,
      'opacity="0.9"',
    ),
    boil(
      (rr) =>
        pencil(
          outline(tube, rr, { roughness: 0.4, double: false, closed: false }) +
            ellipse(cx, bulbY, 15, 15, rr, { double: false, roughness: 0.3 }),
          "ink",
          1.4,
        ),
      seed,
    ),
    feelsTab,
  ].join("");
  const buoy = floating
    ? `<g>${path(
        cut(
          [
            [cx - 26, bulbY - 2],
            [cx + 26, bulbY - 2],
            [cx + 20, bulbY + 18],
            [cx - 20, bulbY + 18],
          ],
          r,
          0.5,
        ),
        "stamp",
      )}${path(cutRect(cx - 25, bulbY + 4, 50, 6, r, 0.3), "white")}${pencil(
        outline(
          [
            [cx - 26, bulbY - 2],
            [cx + 26, bulbY - 2],
            [cx + 20, bulbY + 18],
            [cx - 20, bulbY + 18],
          ],
          r,
          { roughness: 0.4, double: false },
        ),
        "ink",
        1.2,
      )}</g>`
    : "";
  return [
    {
      x: x - cx,
      y: TUBE_TOP - 26,
      w,
      h,
      svg: buoy + glass,
      enter: {
        delay: 0.55 + index * 0.25,
        y: -30,
        rotate: index % 2 ? 6 : -6,
        kind: "drop",
      },
      loop: floating ? "cl-bob" : undefined,
      vars: floating ? { bob: "2.4s" } : undefined,
    },
  ];
}

function tag(
  x: number,
  station: ReportStation,
  index: number,
  seed: number,
): Layer {
  const r = rng(seed);
  const w = 140;
  const h = 58;
  const body: Pt[] = [
    [6, 8],
    [w - 6, 8],
    [w - 6, h - 4],
    [6, h - 4],
  ];
  const svg = [
    path(cut(body, r, 0.8), "paper"),
    path(
      tape(w / 2, 8, 46, 14, between(r, -8, 8), r),
      ["tape-amber", "tape-blue", "tape-mint", "tape-orange"][index % 4],
    ),
    `<text class="cl-hand f-ink" x="${w / 2}" y="31" font-size="20" font-weight="700" text-anchor="middle">${station.name}</text>`,
    unitText(
      w / 2,
      50,
      `${f1(station.temp_avg)}°`,
      `${f1(toF(station.temp_avg))}°`,
      18,
      'text-anchor="middle" font-weight="600"',
      "orange",
    ),
  ].join("");
  return {
    x: x - w / 2,
    y: 340,
    w,
    h,
    svg,
    enter: {
      delay: 1.6 + index * 0.3,
      y: 30,
      rotate: between(r, -6, 6),
      kind: "place",
    },
  };
}

function notes(): Layer[] {
  const arrow = boil(
    (rr) =>
      pencil(
        `${curve(
          [
            [150, 30],
            [300, 18],
            [460, 24],
            [600, 14],
          ].map(
            ([px, py]) =>
              [px + between(rr, -1, 1), py + between(rr, -1, 1)] as Pt,
          ),
        )}M590 6L601 14L589 22`,
        "note",
        2,
      ),
    1092,
  );
  return [
    {
      x: 30,
      y: 30,
      w: 640,
      h: 50,
      svg: `<g class="cl-write" style="--write:0.8s"><text class="cl-hand f-note" x="6" y="36" font-size="30" font-weight="700">airport</text></g>${arrow}<g class="cl-write" style="--write:0.6s;--d:1.2s"><text class="cl-hand f-note" x="612" y="26" font-size="30" font-weight="700">sea</text></g>`,
      className: "cl-flat",
      enter: { delay: 0.6, kind: "fade" },
    },
    {
      x: 800,
      y: 6,
      w: 150,
      h: 26,
      svg: `<text class="cl-hand f-note-soft" x="140" y="19" font-size="17" text-anchor="end">(not to scale)</text>`,
      className: "cl-flat",
      enter: { delay: 2.4, kind: "fade" },
    },
  ];
}

// From the airport to the sea: four stations, each with a thermometer that
// fills to its September mean and a tab at its feels-like.
export function transectScene(stations: ReportStation[]): Scene {
  const byName = (name: string) =>
    stations.find((s) => s.name === name) as ReportStation;
  const order: [string, number, number][] = [
    ["Helsinki-Vantaa", 120, 1101],
    ["Kumpula", 330, 1102],
    ["Kaisaniemi", 594, 1103],
    ["Harmaja", 836, 1104],
  ];
  const gusts = [
    [
      [740, 176],
      [790, 168],
      [840, 176],
      [900, 166],
    ],
    [
      [760, 214],
      [820, 206],
      [870, 214],
      [940, 204],
    ],
  ]
    .map((pts, i) =>
      pencil(
        curve(pts as unknown as Pt[]),
        "pencil",
        1.6,
        `pathLength="1" style="--gd:${n(i * 0.6)}s"`,
        "cl-gust",
      ),
    )
    .join("");
  const layers: Layer[] = [
    sky(),
    cloudLayer(260, 40, 150, 60, 1111, 0.1, "28s", "20%"),
    cloudLayer(640, 70, 120, 50, 1112, 0.2, "22s", "-25%"),
    { x: 0, y: 0, w: W, h: H, svg: gusts, className: "cl-flat" },
    land(),
    sea(),
    airport(),
    kumpula(),
    city(),
    ...harmaja(),
    plane(),
    gull(),
    foreground(),
    ...order.flatMap(([name, x, seed], i) =>
      thermometer(x, byName(name), i, seed, name === "Harmaja"),
    ),
    ...order.map(([name, x, seed], i) => tag(x, byName(name), i, seed + 50)),
    ...notes(),
  ];
  return { w: W, h: H, layers };
}
