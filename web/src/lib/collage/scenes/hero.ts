import { cut, cutCircle, cutRect, tape, torn, tornRect } from "../paper";
import { boil, group, type Layer, path, pencil, type Scene } from "../scene";
import { arcPoints, band, grow, leafShape, stevensonScreen } from "../shapes";
import {
  between,
  brush,
  cloudShape,
  cloudTop,
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

const W = 1200;
const H = 520;

function sky(): Layer {
  const r = rng(11);
  const top = tornRect(-10, -10, W + 20, 150, r, {
    amp: 4,
    step: 5,
    torn: [false, false, true, false],
  });
  const patches = [
    hatch(
      [
        [40, 20],
        [300, 20],
        [260, 120],
        [30, 110],
      ],
      r,
      { angle: -32, gap: 9, roughness: 0.6 },
    ),
    hatch(
      [
        [980, 18],
        [1190, 25],
        [1180, 130],
        [1010, 120],
      ],
      r,
      { angle: -32, gap: 9, roughness: 0.6 },
    ),
    hatch(
      [
        [40, 220],
        [260, 205],
        [250, 300],
        [30, 310],
      ],
      r,
      { angle: -30, gap: 11, roughness: 0.6 },
    ),
  ].join("");
  const svg = [
    `<rect class="f-sky" x="0" y="0" width="${W}" height="${H}"/>`,
    path(top, "sky-deep"),
    pencil(patches, "sky-deep", 1.6, 'opacity="0.9"'),
  ].join("");
  return { x: 0, y: 0, w: W, h: H, svg, className: "cl-base" };
}

function sun(): Layer {
  const w = 190;
  const c = 95;
  const rays = Array.from({ length: 12 }, (_, i) => {
    const a = (i / 12) * Math.PI * 2 + 0.2;
    const r0 = 64;
    const r1 = i % 2 ? 82 : 90;
    const rr = rng(300 + i);
    return path(
      brush(
        [
          [c + r0 * Math.cos(a), c + r0 * Math.sin(a)],
          [
            c + ((r0 + r1) / 2) * Math.cos(a + 0.03),
            c + ((r0 + r1) / 2) * Math.sin(a + 0.03),
          ],
          [c + r1 * Math.cos(a), c + r1 * Math.sin(a)],
        ],
        9,
        rr,
        { taper: 0.45 },
      ),
      "sun-deep",
    );
  }).join("");
  const r = rng(21);
  const disc = cutCircle(c, c, 54, r, 26);
  const shade = hatch(ring(c + 10, c + 12, 40, 38, 24), r, {
    angle: 35,
    gap: 5,
    roughness: 0.5,
  });
  const face = boil(
    (rr) =>
      [
        pencil(
          curve([
            [66, 80],
            [74, 88],
            [84, 81],
          ]),
          "ink",
          3.2,
        ),
        pencil(
          curve([
            [104, 79],
            [113, 87],
            [122, 79],
          ]),
          "ink",
          3.2,
        ),
        pencil(
          curve([
            [84, 100],
            [94, 107],
            [105, 100],
          ]),
          "ink",
          2.8,
        ),
        pencil(line([64, 72], [78, 69], rr, { double: false }), "ink", 2),
        pencil(line([108, 68], [122, 72], rr, { double: false }), "ink", 2),
      ].join(""),
    5,
  );
  const cheeks = `<ellipse class="f-orange-soft" cx="64" cy="95" rx="9" ry="5.5" opacity="0.8"/><ellipse class="f-orange-soft" cx="126" cy="93" rx="9" ry="5.5" opacity="0.8"/>`;
  const awake = boil(
    (rr) =>
      [
        `<circle class="f-ink" cx="75" cy="82" r="5.5"/><circle class="f-white" cx="77" cy="80" r="1.8"/>`,
        `<circle class="f-ink" cx="113" cy="81" r="5.5"/><circle class="f-white" cx="115" cy="79" r="1.8"/>`,
        pencil(
          curve([
            [80, 98],
            [94, 112],
            [109, 98],
          ]),
          "ink",
          3,
        ),
        pencil(line([64, 68], [80, 64], rr, { double: false }), "ink", 2),
        pencil(line([106, 63], [122, 67], rr, { double: false }), "ink", 2),
      ].join(""),
    7,
  );
  const svg = [
    group(rays, `class="cl-sun-rays" style="transform-origin:${c}px ${c}px"`),
    `<g class="cl-sun-body">${[
      path(disc, "sun"),
      pencil(shade, "sun-deep", 2.2, 'opacity="0.7"'),
      boil(
        (rr) =>
          pencil(
            ellipse(c + 3, c + 2, 55, 55, rr, { roughness: 0.6 }),
            "sun-deep",
            1.6,
          ),
        6,
      ),
      cheeks,
      `<g class="cl-face-asleep">${face}</g>`,
      `<g class="cl-face-awake">${awake}</g>`,
    ].join("")}</g>`,
  ].join("");
  return {
    x: 792,
    y: 0,
    w,
    h: w,
    svg,
    enter: { delay: 0.25, y: -40, rotate: -40, kind: "swing" },
    loop: "cl-bob",
    vars: { bob: "4.2s" },
  };
}

function cloud(
  seed: number,
  w: number,
  h: number,
  tone: "cloud" | "rain-cloud",
  bumps: number,
): string {
  const r = rng(seed);
  const pts = cloudShape(8, 8, w - 16, h - 16, r, bumps);
  const shade = tone === "cloud" ? "cloud-shade" : "rain-cloud-shade";
  const bottom = band(pts, h - 8 - (h - 16) * 0.38);
  return [
    path(torn(grow(pts, 2.4), r, { amp: 2.2, step: 4 }), "fiber"),
    path(torn(pts, r, { amp: 2, step: 4 }), tone),
    pencil(
      hatch(bottom, r, { angle: -28, gap: 5.5, roughness: 0.5 }),
      shade,
      2.4,
      'opacity="0.85"',
    ),
    boil(
      (rr) =>
        pencil(
          curve(
            cloudTop(pts, 5).map(
              ([x, y]) =>
                [
                  x + 3 + between(rr, -0.8, 0.8),
                  y - 2 + between(rr, -0.8, 0.8),
                ] as Pt,
            ),
          ),
          "ink",
          1.3,
          'opacity="0.6"',
        ),
      seed,
    ),
  ].join("");
}

function clouds(): Layer[] {
  return [
    {
      x: 1035,
      y: 150,
      w: 140,
      h: 62,
      svg: cloud(41, 140, 62, "cloud", 4),
      enter: { delay: 0.45, x: 60 },
      loop: "cl-drift",
      vars: { drift: "26s", dx: "-14%" },
    },
    {
      x: 545,
      y: 22,
      w: 120,
      h: 50,
      svg: cloud(42, 120, 50, "cloud", 3),
      enter: { delay: 0.5, x: -40 },
      loop: "cl-drift",
      vars: { drift: "31s", dx: "18%" },
    },
    {
      x: 70,
      y: 196,
      w: 150,
      h: 60,
      svg: cloud(43, 150, 60, "cloud", 4),
      enter: { delay: 0.55, x: -60 },
      loop: "cl-drift",
      vars: { drift: "23s", dx: "12%" },
    },
  ];
}

function bigCloud(): Layer {
  return {
    x: 690,
    y: 112,
    w: 300,
    h: 130,
    svg: cloud(51, 300, 130, "cloud", 5),
    enter: { delay: 0.35, x: 30, rotate: 4 },
    loop: "cl-drift",
    vars: { drift: "19s", dx: "-5%" },
  };
}

function rainCloud(): Layer {
  const w = 330;
  const h = 140;
  const r = rng(61);
  const pts = cloudShape(8, 8, w - 16, h - 16, r, 6);
  const svg = [
    path(torn(grow(pts, 2.4), r, { amp: 2.2 }), "fiber"),
    path(torn(pts, r, { amp: 2 }), "rain-cloud"),
    pencil(
      hatch(band(pts, h - 8 - (h - 16) * 0.42), r, {
        angle: -28,
        gap: 5,
        roughness: 0.5,
      }),
      "rain-cloud-shade",
      2.6,
    ),
    pencil(
      hatch(
        band(pts, 0).filter((_, i) => i % 2 === 0),
        r,
        { angle: 50, gap: 13, roughness: 0.8 },
      ),
      "cloud",
      1.4,
      'opacity="0.35"',
    ),
    boil(
      (rr) =>
        pencil(
          curve(
            cloudTop(pts, 5).map(
              ([x, y]) =>
                [
                  x - 3 + between(rr, -0.8, 0.8),
                  y + 2 + between(rr, -0.8, 0.8),
                ] as Pt,
            ),
          ),
          "ink",
          1.4,
          'opacity="0.65"',
        ),
      62,
    ),
  ].join("");
  return {
    x: 318,
    y: 30,
    w,
    h,
    svg,
    enter: { delay: 0.3, x: -30, rotate: -5 },
    loop: "cl-drift",
    vars: { drift: "17s", dx: "4%" },
  };
}

function rain(): Layer {
  const w = 270;
  const h = 290;
  const tile = (seed: number, count: number) => {
    const r = rng(seed);
    let d = "";
    for (let i = 0; i < count; i++) {
      const x = between(r, 4, w - 10);
      const y = between(r, 0, h);
      const len = between(r, 12, 22);
      for (const dy of [0, -h])
        d += `M${n(x)} ${n(y + dy)}L${n(x + len * 0.28)} ${n(y + dy + len)}`;
    }
    return d;
  };
  const svg = [
    `<g class="cl-rain-fall" style="--fall:${h}px;--speed:0.9s">${pencil(tile(71, 26), "rain", 2.2)}</g>`,
    `<g class="cl-rain-fall" style="--fall:${h}px;--speed:0.62s">${pencil(tile(72, 18), "rain", 1.5, 'opacity="0.7"')}</g>`,
  ].join("");
  return {
    x: 352,
    y: 134,
    w,
    h,
    svg,
    className: "cl-rain",
    enter: { delay: 1.6, kind: "fade" },
  };
}

function skyline(
  seed: number,
  x0: number,
  y0: number,
  w: number,
  h: number,
  minH: number,
  maxH: number,
  tone: string,
  toneDark: string,
): string {
  const r = rng(seed);
  const pts: Pt[] = [[0, h]];
  const windows: string[] = [];
  let shades = "";
  let x = 0;
  while (x < w) {
    const bw = between(r, 50, 120);
    const bh = between(r, minH, maxH);
    const top = h - bh;
    const roof = r();
    const x1 = Math.min(w, x + bw);
    if (roof < 0.25) {
      pts.push([x, top + 14], [(x + x1) / 2, top - 8], [x1, top + 14]);
    } else if (roof < 0.45) {
      pts.push(
        [x, top],
        [x1 - 14, top],
        [x1 - 14, top - 12],
        [x1 - 6, top - 12],
        [x1 - 6, top],
        [x1, top],
      );
    } else {
      pts.push([x, top], [x1, top]);
    }
    for (let wy = top + 22; wy < h - 14; wy += 16) {
      for (let wx = x + 9; wx < x1 - 12; wx += 15) {
        if (r() < 0.82)
          windows.push(
            `<rect class="f-window" x="${n(wx + between(r, -0.8, 0.8))}" y="${n(wy + between(r, -0.8, 0.8))}" width="6" height="8" opacity="${n(between(r, 0.55, 0.95))}"/>`,
          );
      }
    }
    shades += hatch(
      [
        [x1 - bw * 0.3, top + 4],
        [x1 - 2, top + 4],
        [x1 - 2, h],
        [x1 - bw * 0.3, h],
      ],
      r,
      { angle: 62, gap: 6, roughness: 0.4 },
    );
    x = x1;
  }
  pts.push([w, h]);
  return group(
    [
      path(cut(pts, r, 0.8), tone),
      windows.join(""),
      pencil(shades, toneDark, 1.8, 'opacity="0.8"'),
    ].join(""),
    `transform="translate(${x0} ${y0})"`,
  );
}

function blocks(): Layer[] {
  return [
    {
      x: 0,
      y: 236,
      w: W,
      h: 210,
      svg: skyline(81, 0, 0, W, 210, 80, 150, "block", "block-dark"),
      enter: { delay: 0.6, y: 40, kind: "grow" },
    },
    {
      x: 0,
      y: 318,
      w: W,
      h: 128,
      svg: skyline(82, 0, 0, W, 128, 50, 100, "block-warm", "kraft-dark"),
      enter: { delay: 0.7, y: 40, kind: "grow" },
    },
  ];
}

function dome(
  cx: number,
  base: number,
  rx: number,
  h: number,
  count = 22,
): Pt[] {
  return Array.from({ length: count + 1 }, (_, i) => {
    const u = -1 + (2 * i) / count;
    return [cx + u * rx, base - h * Math.sqrt(1 - u * u)] as Pt;
  });
}

// Helsinki Cathedral from Senate Square: the broad steps, a six-column
// portico under a pediment with apostles on the roofline, the colonnaded
// drum and green copper dome, and the four corner cupolas.
function cathedral(): Layer {
  const r = rng(91);
  const cx = 150;
  const ground = 310;
  const stairTop = 266;
  const cornice = 196;
  const entBottom = 208;
  const capBottom = 214;
  const colBottom = 262;
  const drumTop = 108;
  const drumBottom = 168;
  const domeBase = 104;
  const shade = (poly: Pt[], gap = 4.5, angle = 72) =>
    pencil(hatch(poly, r, { angle, gap, roughness: 0.35 }), "wall-shade", 1.8);

  const cupola = (x: number, base: number, k: number) => {
    const domePts = dome(x, base - 22 * k, 11 * k, 14 * k);
    const columns = [-6, 0, 6]
      .map((dx) =>
        path(
          cutRect(x + dx * k - 1.2 * k, base - 21 * k, 2.4 * k, 18 * k, r, 0.2),
          "white",
        ),
      )
      .join("");
    return [
      path(cutRect(x - 13 * k, base, 26 * k, 26 * k, r, 0.4), "wall"),
      path(
        cutRect(x - 10.5 * k, base - 22 * k, 21 * k, 22 * k, r, 0.3),
        "wall-shade",
      ),
      columns,
      path(cutRect(x - 12 * k, base - 25 * k, 24 * k, 4 * k, r, 0.2), "white"),
      path(
        cut(
          [
            [x - 11 * k, base - 22 * k],
            ...domePts.slice(1, -1),
            [x + 11 * k, base - 22 * k],
          ],
          r,
          0.3,
        ),
        "copper",
      ),
      pencil(
        hatch(
          domePts.filter(([px]) => px <= x + 1).concat([[x, base - 22 * k]]),
          r,
          { angle: 80, gap: 2.6, roughness: 0.3 },
        ),
        "copper-dark",
        1.3,
      ),
      pencil(
        `M${n(x)} ${n(base - 36 * k)}L${n(x)} ${n(base - 46 * k)}M${n(x - 3 * k)} ${n(base - 42.5 * k)}L${n(x + 3 * k)} ${n(base - 42.5 * k)}`,
        "sun-deep",
        1.6 * k,
      ),
      `<circle class="f-sun" cx="${n(x)}" cy="${n(base - 36 * k)}" r="${n(1.8 * k)}"/>`,
    ].join("");
  };

  const statue = (x: number, foot: number, k = 1) =>
    path(
      cut(
        [
          [x - 2.6 * k, foot],
          [x - 1.8 * k, foot - 7 * k],
          [x - 1.2 * k, foot - 8.4 * k],
          [x + 1.2 * k, foot - 8.4 * k],
          [x + 1.8 * k, foot - 7 * k],
          [x + 2.6 * k, foot],
        ],
        r,
        0.15,
      ),
      "slate",
    ) +
    `<circle class="f-slate" cx="${n(x)}" cy="${n(foot - 10 * k)}" r="${n(1.7 * k)}"/>`;

  const stairs: string[] = [
    path(
      cut(
        [
          [6, ground],
          [294, ground],
          [278, stairTop],
          [22, stairTop],
        ],
        r,
        0.5,
      ),
      "wall-shade",
    ),
  ];
  for (let i = 1; i < 11; i++) {
    const y = stairTop + ((ground - stairTop) * i) / 11;
    const t = (y - stairTop) / (ground - stairTop);
    stairs.push(
      path(cutRect(22 - 16 * t, y - 1.8, 256 + 32 * t, 1.8, r, 0.2), "white"),
    );
  }
  stairs.push(
    pencil(
      hatch(
        [
          [6, ground],
          [58, ground],
          [62, stairTop],
          [22, stairTop],
        ],
        r,
        { angle: 70, gap: 4, roughness: 0.3 },
      ),
      "granite",
      1.2,
      'opacity="0.5"',
    ),
  );

  const drumX0 = 104;
  const drumX1 = 196;
  const drumR = (drumX1 - drumX0) / 2;
  const drumCols = [-70, -48, -24, 0, 24, 48, 70].map((deg) => {
    const a = (deg * Math.PI) / 180;
    const x = cx + drumR * Math.sin(a);
    const w = 4.6 * Math.cos(a) + 1;
    return path(
      cutRect(x - w / 2, drumTop + 4, w, drumBottom - drumTop - 6, r, 0.2),
      "white",
    );
  });
  const drumWindows = [-36, -12, 12, 36].map((deg) => {
    const a = (deg * Math.PI) / 180;
    const x = cx + drumR * Math.sin(a);
    const w = 7 * Math.cos(a);
    return path(
      cut(
        [
          [x - w / 2, drumBottom - 12],
          [x - w / 2, drumTop + 26],
          ...arcPoints(x, drumTop + 26, w / 2, Math.PI, 2 * Math.PI, 6),
          [x + w / 2, drumBottom - 12],
        ],
        r,
        0.2,
      ),
      "granite-dark",
    );
  });
  const mainDome = dome(cx, domeBase, 47, 54);
  const seams = [-62, -40, -20, 0, 20, 40, 62]
    .map((deg) => {
      const a = (deg * Math.PI) / 180;
      return curve(
        Array.from({ length: 7 }, (_, i) => {
          const v = (i / 6) * 0.93;
          return [
            cx + 47 * Math.sqrt(1 - v * v) * Math.sin(a),
            domeBase - 54 * v,
          ] as Pt;
        }),
      );
    })
    .join("");
  const lanternTop = domeBase - 54 - 18;
  const lanternDome = dome(cx, lanternTop, 9.5, 10);

  const columns = [100, 120, 140, 160, 180, 200]
    .map((x) =>
      [
        path(
          cutRect(x - 4, capBottom, 8, colBottom - capBottom, r, 0.2),
          "white",
        ),
        path(
          cutRect(x - 4, capBottom, 2.6, colBottom - capBottom, r, 0.1),
          "wall-shade",
        ),
        path(
          cut(
            [
              [x - 4.5, capBottom],
              [x + 4.5, capBottom],
              [x + 6.5, entBottom],
              [x - 6.5, entBottom],
            ],
            r,
            0.2,
          ),
          "white",
        ),
        pencil(
          `M${x - 6.5} ${entBottom}L${x + 6.5} ${entBottom}M${x - 3} ${capBottom - 3}L${x + 3} ${capBottom - 3}`,
          "wall-shade",
          1,
        ),
        path(
          cutRect(x - 5.6, colBottom, 11.2, stairTop - colBottom, r, 0.2),
          "white",
        ),
      ].join(""),
    )
    .join("");

  const ink = (rr: Rnd) =>
    [
      outline(
        [
          [40, stairTop],
          [40, cornice],
          [260, cornice],
          [260, stairTop],
        ],
        rr,
        { roughness: 0.35, double: false, closed: false },
      ),
      outline(
        [
          [88, cornice],
          [cx, cornice - 29],
          [212, cornice],
        ],
        rr,
        { roughness: 0.35, double: false, closed: false },
      ),
      line([92, entBottom], [208, entBottom], rr, {
        roughness: 0.3,
        double: false,
      }),
      outline(
        [
          [drumX0, drumBottom],
          [drumX0, drumTop + 3],
        ],
        rr,
        { roughness: 0.3, double: false, closed: false },
      ),
      outline(
        [
          [drumX1, drumTop + 3],
          [drumX1, drumBottom],
        ],
        rr,
        { roughness: 0.3, double: false, closed: false },
      ),
      curve(
        mainDome
          .filter((_, i) => i % 2 === 0)
          .map(
            ([x, y]) =>
              [x + between(rr, -0.5, 0.5), y + between(rr, -0.5, 0.5)] as Pt,
          ),
      ),
      line([22, stairTop], [278, stairTop], rr, {
        roughness: 0.3,
        double: false,
      }),
    ].join("");

  const svg = [
    cupola(90, cornice - 14, 0.82),
    cupola(210, cornice - 14, 0.82),
    path(cutRect(92, drumBottom, 116, 16, r, 0.4), "wall"),
    path(cutRect(88, drumBottom - 4, 124, 6, r, 0.3), "white"),
    path(
      cutRect(
        drumX0,
        drumTop + 2,
        drumX1 - drumX0,
        drumBottom - drumTop,
        r,
        0.4,
      ),
      "wall-shade",
    ),
    ...drumWindows,
    ...drumCols,
    shade(
      [
        [drumX0 + 1, drumTop + 4],
        [128, drumTop + 4],
        [128, drumBottom - 2],
        [drumX0 + 1, drumBottom - 2],
      ],
      4,
      75,
    ),
    path(
      cutRect(drumX0 - 5, domeBase - 3, drumX1 - drumX0 + 10, 8, r, 0.3),
      "white",
    ),
    pencil(
      `M${drumX0 - 4} ${domeBase + 5}L${drumX1 + 4} ${domeBase + 5}`,
      "wall-shade",
      1.4,
    ),
    path(
      cut(
        [
          [drumX0 - 1, domeBase],
          ...mainDome.slice(1, -1),
          [drumX1 + 1, domeBase],
        ],
        r,
        0.4,
      ),
      "copper",
    ),
    pencil(
      hatch(
        [...mainDome.filter(([x]) => x <= cx - 12), [cx - 12, domeBase]],
        r,
        { angle: 82, gap: 3.6, roughness: 0.35 },
      ),
      "copper-dark",
      1.6,
    ),
    pencil(seams, "copper-dark", 1.2, 'opacity="0.75"'),
    pencil(
      curve([
        [cx + 22, domeBase - 44],
        [cx + 34, domeBase - 32],
        [cx + 40, domeBase - 14],
      ]),
      "white",
      2.4,
      'opacity="0.35"',
    ),
    path(cutRect(cx - 9, lanternTop, 18, 20, r, 0.3), "wall"),
    pencil(
      `M${cx - 4} ${lanternTop + 2}L${cx - 4} ${lanternTop + 18}M${cx + 1} ${lanternTop + 2}L${cx + 1} ${lanternTop + 18}M${cx + 6} ${lanternTop + 2}L${cx + 6} ${lanternTop + 18}`,
      "wall-shade",
      1.4,
    ),
    path(cutRect(cx - 11, lanternTop - 2, 22, 4, r, 0.2), "white"),
    path(
      cut(
        [
          [cx - 9.5, lanternTop],
          ...lanternDome.slice(1, -1),
          [cx + 9.5, lanternTop],
        ],
        r,
        0.2,
      ),
      "copper",
    ),
    pencil(
      `M${cx} ${lanternTop - 9}L${cx} ${lanternTop - 34}M${cx - 6} ${lanternTop - 25}L${cx + 6} ${lanternTop - 25}`,
      "sun-deep",
      2.6,
    ),
    `<circle class="f-sun" cx="${cx}" cy="${lanternTop - 10}" r="2.6"/>`,
    path(cutRect(44, cornice - 16, 212, 18, r, 0.4), "wall-shade"),
    cupola(56, cornice - 6, 1),
    cupola(244, cornice - 6, 1),
    path(cutRect(40, cornice, 220, stairTop - cornice, r, 0.5), "wall"),
    shade(
      [
        [42, cornice + 4],
        [94, cornice + 4],
        [94, stairTop - 2],
        [42, stairTop - 2],
      ],
      5,
      74,
    ),
    ...[58, 76, 224, 242].map((x) =>
      path(
        cut(
          [
            [x - 4.5, stairTop - 12],
            [x - 4.5, cornice + 28],
            ...arcPoints(x, cornice + 28, 4.5, Math.PI, 2 * Math.PI, 6),
            [x + 4.5, stairTop - 12],
          ],
          r,
          0.2,
        ),
        "granite-dark",
      ),
    ),
    pencil(
      `M46 ${cornice + 6}L46 ${stairTop}M88 ${cornice + 6}L88 ${stairTop}M212 ${cornice + 6}L212 ${stairTop}M254 ${cornice + 6}L254 ${stairTop}`,
      "wall-shade",
      1.6,
    ),
    path(cutRect(36, cornice - 7, 228, 8, r, 0.3), "white"),
    pencil(`M36 ${cornice + 1.5}L264 ${cornice + 1.5}`, "wall-shade", 1.8),
    path(
      cutRect(94, entBottom, 112, stairTop - entBottom, r, 0.3),
      "wall-shade",
    ),
    path(
      cut(
        [
          [141, stairTop],
          [141, stairTop - 28],
          ...arcPoints(150, stairTop - 28, 9, Math.PI, 2 * Math.PI, 8),
          [159, stairTop],
        ],
        r,
        0.2,
      ),
      "trunk",
    ),
    columns,
    path(cutRect(90, cornice, 120, entBottom - cornice, r, 0.3), "white"),
    pencil(`M92 ${cornice + 6}L208 ${cornice + 6}`, "wall-shade", 1.1),
    path(
      cut(
        [
          [86, cornice],
          [cx, cornice - 29],
          [214, cornice],
        ],
        r,
        0.3,
      ),
      "white",
    ),
    path(
      cut(
        [
          [100, cornice - 2],
          [cx, cornice - 22],
          [200, cornice - 2],
        ],
        r,
        0.2,
      ),
      "wall",
    ),
    pencil(
      hatch(
        [
          [100, cornice - 2],
          [cx, cornice - 22],
          [cx, cornice - 2],
        ],
        r,
        { angle: 70, gap: 3.5, roughness: 0.3 },
      ),
      "wall-shade",
      1.2,
    ),
    statue(cx, cornice - 29, 1.1),
    statue(89, cornice - 7),
    statue(211, cornice - 7),
    statue(39, cornice - 7, 0.9),
    statue(261, cornice - 7, 0.9),
    ...stairs,
    boil((rr) => pencil(ink(rr), "ink", 1.3, 'opacity="0.85"'), 92),
  ].join("");
  return {
    x: 356,
    y: 114,
    w: 300,
    h: 310,
    svg,
    enter: { delay: 0.85, y: 30, kind: "grow" },
  };
}

function station(): Layer[] {
  const r = rng(101);
  const tower = [
    path(cutRect(30, 95, 50, 300, r, 0.7), "granite"),
    pencil(
      hatch(
        [
          [64, 98],
          [78, 98],
          [78, 392],
          [64, 392],
        ],
        r,
        { angle: 75, gap: 5, roughness: 0.4 },
      ),
      "granite-dark",
      2,
    ),
    ...[40, 50, 60, 70].map((x) =>
      pencil(
        line([x, 140], [x, 380], r, { roughness: 0.3, double: false }),
        "granite-dark",
        1.4,
        'opacity="0.8"',
      ),
    ),
    path(cutRect(25, 72, 60, 25, r, 0.5), "granite-dark"),
    path(cutRect(33, 54, 44, 19, r, 0.5), "granite"),
    path(cutRect(40, 41, 30, 14, r, 0.4), "granite-dark"),
    path(
      cut(
        [
          [40, 42],
          [55, 9],
          [70, 42],
        ],
        r,
        0.4,
      ),
      "copper",
    ),
    pencil(
      hatch(
        [
          [56, 14],
          [69, 42],
          [57, 42],
        ],
        r,
        { angle: 80, gap: 3.5, roughness: 0.3 },
      ),
      "copper-dark",
      1.6,
    ),
    pencil("M55 9L55 0", "ink", 1.6),
    `<circle class="f-paper" cx="55" cy="118" r="13"/>`,
    boil(
      (rr) =>
        pencil(
          ellipse(55, 118, 13, 13, rr, { roughness: 0.4, double: false }) +
            outline(
              [
                [30, 395],
                [30, 95],
                [80, 95],
                [80, 395],
              ],
              rr,
              { roughness: 0.4, double: false, closed: false },
            ),
          "ink",
          1.4,
        ),
      102,
    ),
    `<g class="cl-clock-hour" style="transform-origin:55px 118px">${pencil("M55 118L55 111", "ink", 2.4)}</g>`,
    `<g class="cl-clock-minute" style="transform-origin:55px 118px">${pencil("M55 118L63 118", "ink", 1.6)}</g>`,
    `<circle class="f-ink" cx="55" cy="118" r="1.6"/>`,
  ].join("");
  const hallW = 250;
  const archPts: Pt[] = [
    [82, 115],
    [82, 82],
    ...arcPoints(125, 82, 43, Math.PI, 2 * Math.PI, 14),
    [168, 115],
  ];
  const mullions = [
    ...[96, 110, 125, 140, 154].map((x) =>
      line([x, 115], [x, 45], r, { roughness: 0.3, double: false }),
    ),
    ...[70, 92].map((y) =>
      line([82, y], [168, y], r, { roughness: 0.3, double: false }),
    ),
  ].join("");
  const bearer = (cx: number, lampSide: number) =>
    [
      path(
        cut(
          [
            [cx - 9, 112],
            [cx - 7, 62],
            [cx - 4, 54],
            [cx + 4, 54],
            [cx + 7, 62],
            [cx + 9, 112],
          ],
          r,
          0.4,
        ),
        "granite-dark",
      ),
      `<circle class="f-granite-dark" cx="${cx}" cy="48" r="7"/>`,
      pencil(
        curve([
          [cx + lampSide * 4, 62],
          [cx + lampSide * 11, 70],
          [cx + lampSide * 14, 66],
        ]),
        "granite-dark",
        4,
      ),
      `<circle class="f-sun cl-glow" cx="${cx + lampSide * 15}" cy="70" r="10" opacity="0.35"/>`,
      `<circle class="f-window" cx="${cx + lampSide * 15}" cy="70" r="5.5"/>`,
    ].join("");
  const hall = [
    path(
      cut(
        [
          [0, 115],
          [0, 38],
          [60, 30],
          [125, 24],
          [190, 30],
          [250, 38],
          [250, 115],
        ],
        r,
        0.8,
      ),
      "granite",
    ),
    pencil(
      hatch(
        [
          [200, 40],
          [248, 42],
          [248, 113],
          [200, 113],
        ],
        r,
        { angle: 70, gap: 5, roughness: 0.4 },
      ),
      "granite-dark",
      1.8,
    ),
    path(cut(archPts, r, 0.4), "glass"),
    pencil(mullions, "pencil", 1.2),
    ...[12, 30, 210, 228].map((x) =>
      path(cutRect(x, 60, 9, 26, r, 0.3), "sky-deep"),
    ),
    bearer(62, -1),
    bearer(188, 1),
    boil(
      (rr) =>
        pencil(
          outline(
            archPts.filter((_, i) => i % 2 === 0),
            rr,
            { roughness: 0.4, double: false, closed: false },
          ),
          "ink",
          1.3,
        ),
      103,
    ),
  ].join("");
  return [
    {
      x: 655,
      y: 52,
      w: 110,
      h: 395,
      svg: tower,
      enter: { delay: 0.95, y: 26, kind: "grow" },
    },
    {
      x: 690,
      y: 330,
      w: hallW,
      h: 115,
      svg: hall,
      enter: { delay: 1.05, y: 30, kind: "grow" },
    },
  ];
}

function trees(): Layer {
  const r = rng(111);
  const tree = (
    cx: number,
    cy: number,
    rx: number,
    ry: number,
    tone: string,
    hatchTone: string,
    trunkTone: string,
    seed: number,
    birch: boolean,
  ) => {
    const rr = rng(seed);
    const crown = ring(cx, cy, rx, ry, 30, rr() * 6).map(
      ([x, y]) => [x + between(rr, -3, 3), y + between(rr, -3, 3)] as Pt,
    );
    const trunk = path(
      cut(
        [
          [cx - 5, 195],
          [cx - 3.5, cy + ry * 0.4],
          [cx + 3.5, cy + ry * 0.4],
          [cx + 5, 195],
        ],
        rr,
        0.4,
      ),
      trunkTone,
    );
    const marks = birch
      ? pencil(
          [0, 1, 2, 3, 4]
            .map((i) => {
              const y = cy + ry * 0.55 + i * 15 + between(rr, -3, 3);
              return `M${n(cx - 4)} ${n(y)}L${n(cx + between(rr, -1, 2))} ${n(y + 1.5)}`;
            })
            .join(""),
          "ink",
          2.2,
        )
      : "";
    const speckles = Array.from({ length: 14 }, () => {
      const a = rr() * 6.28;
      const k = Math.sqrt(rr()) * 0.85;
      return `<ellipse class="f-${hatchTone}" cx="${n(cx + rx * k * Math.cos(a))}" cy="${n(cy + ry * k * Math.sin(a))}" rx="3.2" ry="2" transform="rotate(${n(rr() * 180)} ${n(cx + rx * k * Math.cos(a))} ${n(cy + ry * k * Math.sin(a))})"/>`;
    }).join("");
    return group(
      [
        trunk,
        marks,
        path(torn(grow(crown, 2.2), rr, { amp: 2.4 }), "fiber"),
        path(torn(crown, rr, { amp: 2.2 }), tone),
        pencil(
          hatch(
            crown.filter((_, i) => i < 18),
            rr,
            { angle: -50, gap: 6, roughness: 0.6 },
          ),
          hatchTone,
          2,
          'opacity="0.55"',
        ),
        speckles,
        boil(
          (b) =>
            pencil(
              ellipse(cx - 2, cy + 1, rx * 0.98, ry * 0.98, b, {
                roughness: 1.4,
                double: false,
              }),
              "ink",
              1.2,
              'opacity="0.5"',
            ),
          seed,
        ),
      ].join(""),
      `class="cl-sway" style="transform-origin:${cx}px 195px;--sway:${n(between(r, 2.2, 3.4))}s"`,
    );
  };
  const svg = [
    tree(212, 98, 46, 54, "leaf-green", "grass-dark", "trunk", 113, false),
    tree(48, 76, 40, 56, "birch", "sun-deep", "bark", 112, true),
    tree(128, 86, 60, 58, "maple", "rowan", "trunk", 114, false),
  ].join("");
  return {
    x: 925,
    y: 248,
    w: 265,
    h: 195,
    svg,
    enter: { delay: 1.15, y: 30, kind: "grow" },
  };
}

function screen(): Layer {
  const r = rng(121);
  const svg = [
    stevensonScreen(0, 0, 1, 122),
    pencil(
      curve([
        [61, 22],
        [70, 30],
        [76, 40],
      ]),
      "ink",
      1,
    ),
    `<g transform="rotate(14 82 50)">${path(
      cut(
        [
          [73, 40],
          [92, 40],
          [92, 60],
          [73, 60],
          [70, 50],
        ],
        r,
        0.4,
      ),
      "kraft",
    )}<circle class="f-paper" cx="76" cy="47" r="1.6"/>${pencil("M78 52L88 52M78 55L86 55", "ink", 0.9)}</g>`,
  ].join("");
  return {
    x: 1010,
    y: 356,
    w: 100,
    h: 90,
    svg,
    enter: { delay: 1.3, y: -60, kind: "drop" },
  };
}

function ground(): Layer {
  const r = rng(131);
  const lawn = tornRect(-10, 8, W + 20, 190, r, {
    amp: 3.5,
    step: 5,
    torn: [true, false, false, false],
  });
  const pathStrip = tornRect(-10, 42, W + 20, 26, r, {
    amp: 2.2,
    step: 5,
    torn: [true, false, true, false],
  });
  const tufts = Array.from({ length: 34 }, () => {
    const x = between(r, 10, W - 10);
    const y = between(r, 74, 100);
    return `M${n(x)} ${n(y)}l${n(between(r, -3, -1))} ${n(-between(r, 5, 8))}M${n(x + 2)} ${n(y)}l${n(between(r, 0, 2))} ${n(-between(r, 6, 10))}`;
  }).join("");
  const svg = [
    path(lawn, "grass"),
    pencil(
      hatch(
        [
          [0, 14],
          [W, 14],
          [W, 40],
          [0, 40],
        ],
        r,
        { angle: -60, gap: 7, roughness: 0.5 },
      ),
      "grass-dark",
      2,
      'opacity="0.6"',
    ),
    path(pathStrip, "path"),
    pencil(
      hatch(
        [
          [0, 46],
          [W, 46],
          [W, 66],
          [0, 66],
        ],
        r,
        { angle: 15, gap: 7, roughness: 0.6 },
      ),
      "kraft-dark",
      1.2,
      'opacity="0.5"',
    ),
    pencil(tufts, "grass-dark", 1.6),
  ].join("");
  return {
    x: 0,
    y: 412,
    w: W,
    h: 170,
    svg,
    enter: { delay: 0.75, y: 50, kind: "grow" },
  };
}

function branch(): Layer {
  const r = rng(141);
  const main: Pt[] = [
    [-6, 14],
    [60, 26],
    [140, 46],
    [215, 70],
    [282, 98],
    [318, 118],
  ];
  const fork: Pt[] = [
    [118, 41],
    [170, 30],
    [222, 26],
  ];
  const tones = ["birch", "birch", "sun", "leaf-green", "birch", "birch"];
  const twigs: Pt[][] = [];
  const leaves: string[] = [];
  const anchors: [Pt, number][] = [
    [[30, 21], 58],
    [[74, 30], 104],
    [[104, 37], 72],
    [[150, 49], 120],
    [[186, 29], 64],
    [[196, 61], 92],
    [[244, 81], 70],
    [[276, 95], 48],
  ];
  anchors.forEach(([base, len], i) => {
    const drift = between(r, 4, 20);
    const curl = between(r, -6, 8);
    const tw: Pt[] = [
      base,
      [base[0] + drift * 0.35 + curl, base[1] + len * 0.35],
      [base[0] + drift * 0.8, base[1] + len * 0.72],
      [base[0] + drift, base[1] + len],
    ];
    twigs.push(tw);
    const count = 2 + (i % 3);
    for (let k = 0; k < count; k++) {
      const u = 0.32 + (0.66 * (k + between(r, 0, 0.5))) / count;
      const side = k % 2 ? 1 : -1;
      leaves.push(
        leafShape(
          base[0] + drift * u + side * between(r, 4, 8),
          base[1] + len * u + between(r, -3, 3),
          between(r, 6, 9),
          side * between(r, 20, 50),
          tones[(i * 3 + k) % tones.length],
          r,
        ),
      );
    }
  });
  for (const [x, y] of [
    [48, 24],
    [128, 44],
    [204, 26],
    [262, 88],
  ] as Pt[]) {
    leaves.push(
      leafShape(x, y - 7, between(r, 6, 8), between(r, -60, -20), "birch", r),
    );
  }
  const svg = [
    path(brush(main, 10, r, { taper: 0.5, wobble: 0.15 }), "trunk"),
    path(brush(fork, 4, r, { taper: 0.6, wobble: 0.2 }), "trunk"),
    ...twigs.map((tw) =>
      path(brush(tw, 2.4, r, { taper: 0.7, wobble: 0.2 }), "trunk"),
    ),
    leaves.join(""),
  ].join("");
  return {
    x: -8,
    y: -6,
    w: 330,
    h: 210,
    svg,
    enter: { delay: 1.35, x: -30, y: -30, rotate: -12, kind: "swing" },
    loop: "cl-branch",
  };
}

function fallingLeaves(): Layer[] {
  const r = rng(151);
  const tones = ["birch", "maple", "birch", "sun", "rowan", "birch", "maple"];
  return tones.map((tone, i) => {
    const size = 24;
    const svg = leafShape(12, 12, 9, between(r, -30, 30), tone, r);
    return {
      x: between(r, 60, 280),
      y: between(r, 60, 150),
      w: size,
      h: size,
      svg,
      className: "cl-leaf",
      vars: {
        fall: `${n(between(r, 1300, 1700))}%`,
        drift: `${n(between(r, 900, 2400))}%`,
        dur: `${n(between(r, 9, 14))}s`,
        delay: `${n(1.8 + i * 1.7 + between(r, 0, 1))}s`,
        spin: `${n(between(r, 1.1, 1.8))}s`,
      },
    } satisfies Layer;
  });
}

function walker(): Layer {
  const legs: [Pt, Pt, number][] = [
    [[44, 97], [26, 97], 0],
    [[37, 95], [33, 97], -1.2],
    [[26, 97], [44, 97], 0],
    [[33, 97], [37, 95], -1.2],
  ];
  const frames = legs
    .map(([front, back, lift], i) => {
      const rr = rng(160 + i);
      const hip: Pt = [35, 76 + lift];
      return `<g class="cl-fr cl-fr${i}">${[
        pencil(
          curve([
            hip,
            [(hip[0] + back[0]) / 2 - 1, (hip[1] + back[1]) / 2],
            back,
          ]),
          "ink",
          3.6,
        ),
        pencil(
          curve([
            hip,
            [(hip[0] + front[0]) / 2 + 1, (hip[1] + front[1]) / 2 - 1],
            front,
          ]),
          "ink",
          3.6,
        ),
        `<ellipse class="f-ink" cx="${n(back[0] + 2)}" cy="${n(back[1])}" rx="4" ry="2.2"/>`,
        `<ellipse class="f-ink" cx="${n(front[0] + 2)}" cy="${n(front[1])}" rx="4" ry="2.2"/>`,
        `<g transform="translate(0 ${lift})">${[
          path(
            cut(
              [
                [27, 51],
                [44, 51],
                [49, 80],
                [22, 80],
              ],
              rr,
              0.6,
            ),
            "coat",
          ),
          pencil(
            outline(
              [
                [27, 51],
                [44, 51],
                [49, 80],
                [22, 80],
              ],
              rr,
              { roughness: 0.5, double: false },
            ),
            "ink",
            1.2,
          ),
          `<circle class="f-coat" cx="33" cy="44" r="8.8"/>`,
          `<circle class="f-skin" cx="35.6" cy="45.2" r="5.6"/>`,
          `<circle class="f-ink" cx="38.2" cy="44.2" r="0.95"/>`,
          `<ellipse class="f-orange-soft" cx="37.6" cy="47.6" rx="1.8" ry="1.1" opacity="0.8"/>`,
          pencil(
            curve([
              [34, 55],
              [39, 60],
              [44, 58],
            ]),
            "coat",
            4.4,
          ),
          `<circle class="f-skin" cx="44.4" cy="57.6" r="2.4"/>`,
        ].join("")}</g>`,
      ].join("")}</g>`;
    })
    .join("");
  const r = rng(170);
  const cx = 44;
  const canopy: Pt[] = [
    [cx - 31, 26],
    ...arcPoints(cx, 26, 31, Math.PI, 2 * Math.PI, 12).map(
      ([x, y]) => [x, y + (y - 26) * -0.15] as Pt,
    ),
    [cx + 31, 26],
    [cx + 21, 22],
    [cx + 11, 27],
    [cx, 22],
    [cx - 11, 27],
    [cx - 21, 22],
  ];
  const umbrella = [
    pencil(`M${cx} 4L${cx} 58`, "ink", 1.7),
    pencil(
      curve([
        [cx, 58],
        [cx, 63],
        [cx - 4, 63],
        [cx - 5, 60],
      ]),
      "ink",
      1.7,
    ),
    path(cut(canopy, r, 0.6), "umbrella"),
    pencil(
      hatch(canopy.slice(1, 8), r, { angle: 60, gap: 4.5, roughness: 0.4 }),
      "rowan",
      1.6,
    ),
    pencil(
      curve([
        [cx, -4],
        [cx, 2],
      ]),
      "ink",
      1.6,
    ),
    pencil(
      curve([
        [cx, 0],
        [cx - 15, 10],
        [cx - 21, 22],
      ]) +
        curve([
          [cx, 0],
          [cx + 15, 10],
          [cx + 21, 22],
        ]),
      "rowan",
      1.2,
    ),
  ].join("");
  const svg = `${frames}<g class="cl-umbrella" style="transform-origin:${cx}px 58px">${umbrella}</g>`;
  return { x: -110, y: 372, w: 72, h: 100, svg, className: "cl-walker" };
}

function scrap(): Layer {
  const r = rng(181);
  const w = 320;
  const h = 64;
  const body = tornRect(4, 6, w - 8, h - 12, r, {
    amp: 2.4,
    torn: [false, true, false, true],
  });
  const rules = [24, 38, 52]
    .map((y) => line([8, y], [w - 8, y], r, { roughness: 0.2, double: false }))
    .join("");
  const svg = group(
    [
      path(
        tornRect(2, 4, w - 4, h - 8, r, {
          amp: 3,
          torn: [false, true, false, true],
        }),
        "fiber",
      ),
      path(body, "notebook"),
      pencil(rules, "rule", 1),
      pencil(
        line([30, 8], [30, h - 8], r, { roughness: 0.2, double: false }),
        "margin",
        1.2,
      ),
      `<g class="cl-write">${`<text class="cl-hand f-ink" x="${(w + 30) / 2}" y="44" font-size="28" font-weight="600" text-anchor="middle">Helsinki, September 2026</text>`}</g>`,
      path(tape(14, 14, 44, 16, -38, r), "tape-amber"),
      path(tape(w - 12, h - 14, 40, 15, -30, r), "tape-blue"),
    ].join(""),
    `transform="rotate(-2.5 ${w / 2} ${h / 2})"`,
  );
  return {
    x: 440,
    y: 444,
    w,
    h,
    svg,
    enter: { delay: 1.55, y: 40, rotate: 9, kind: "drop" },
  };
}

function note(
  x: number,
  y: number,
  w: number,
  h: number,
  label: string,
  arrow: Pt[],
  seed: number,
  delay: number,
  tilt: number,
): Layer {
  const svg = [
    `<g class="cl-write" style="--write:0.7s">${`<text class="cl-hand f-note" x="4" y="${h * 0.42}" font-size="32" font-weight="700" transform="rotate(${tilt} 4 ${h * 0.42})">${label}</text>`}</g>`,
    boil((rr) => {
      const tip = arrow.at(-1) as Pt;
      const before = arrow.at(-2) as Pt;
      const a = Math.atan2(tip[1] - before[1], tip[0] - before[0]);
      const head = [a + 2.6, a - 2.6]
        .map(
          (b) =>
            `M${n(tip[0])} ${n(tip[1])}L${n(tip[0] + 11 * Math.cos(b) + between(rr, -1, 1))} ${n(tip[1] + 11 * Math.sin(b) + between(rr, -1, 1))}`,
        )
        .join("");
      return pencil(
        curve(
          arrow.map(
            ([px, py]) =>
              [px + between(rr, -1, 1), py + between(rr, -1, 1)] as Pt,
          ),
        ) + head,
        "note",
        2.1,
        'pathLength="1"',
        "cl-draw",
      );
    }, seed),
  ].join("");
  return { x, y, w, h, svg, enter: { delay, kind: "fade" } };
}

function at(depth: number, ...layers: Layer[]): Layer[] {
  return layers.map((layer) => ({ ...layer, depth }));
}

export function heroScene(): Scene {
  const layers: Layer[] = [
    sky(),
    ...at(0.06, sun()),
    ...at(0.1, ...clouds()),
    ...at(0.14, bigCloud(), rain(), rainCloud()),
    ...at(0.2, blocks()[0]),
    ...at(0.25, blocks()[1]),
    ...at(0.3, cathedral(), ...station()),
    ...at(0.34, trees()),
    ...at(0.4, ground(), screen(), walker()),
    ...at(0.55, branch(), ...fallingLeaves()),
    ...at(0.46, scrap()),
    ...at(
      0.18,
      note(
        236,
        214,
        120,
        80,
        "wet!",
        [
          [52, 40],
          [80, 30],
          [100, -30],
        ],
        191,
        1.9,
        -6,
      ),
      note(
        950,
        28,
        120,
        80,
        "mild",
        [
          [8, 44],
          [-8, 50],
          [-26, 62],
        ],
        192,
        2.1,
        4,
      ),
    ),
  ];
  return { w: W, h: H, layers };
}
