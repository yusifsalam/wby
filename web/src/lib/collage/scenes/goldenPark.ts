import { cut, cutCircle, cutRect, torn, tornRect } from "../paper";
import { boil, group, type Layer, path, pencil, type Scene } from "../scene";
import { arcPoints, leafShape, stevensonScreen } from "../shapes";
import {
  between,
  brush,
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
  sample,
} from "../sketch";
import { at, note, scrap, skyline } from "./hero";

const W = 1200;
const H = 520;
const LEAF_TONES = ["maple", "birch", "rowan", "sun", "maple", "birch"];
const LAWN_Y = 360;

// The park path's upper edge at sample i, in lawn coordinates; samples are
// 102 units apart from x = -10 and the path is 30 units wide.
function walkY(i: number): number {
  return 62 - i * 3.4 + Math.sin(i * 0.8) * 6;
}

// The middle of the path at scene x, in scene coordinates.
function pathMiddle(x: number): number {
  const u = (x + 10) / 102;
  const i = Math.max(0, Math.min(11, Math.floor(u)));
  const t = u - i;
  return LAWN_Y + 15 + walkY(i) * (1 - t) + walkY(i + 1) * t;
}

function wobble(points: Pt[], rr: Rnd, by = 0.8): Pt[] {
  return points.map(
    ([x, y]) => [x + between(rr, -by, by), y + between(rr, -by, by)] as Pt,
  );
}

function sky(): Layer {
  const r = rng(2011);
  const glow = tornRect(-10, 146, W + 20, 150, r, {
    amp: 5,
    step: 6,
    torn: [true, false, false, false],
  });
  const glowDeep = tornRect(-10, 212, W + 20, 90, r, {
    amp: 4,
    step: 6,
    torn: [true, false, false, false],
  });
  const patches = [
    hatch(
      [
        [30, 18],
        [320, 14],
        [290, 110],
        [24, 120],
      ],
      r,
      { angle: -30, gap: 10, roughness: 0.6 },
    ),
    hatch(
      [
        [560, 10],
        [800, 16],
        [780, 90],
        [570, 84],
      ],
      r,
      { angle: -30, gap: 10, roughness: 0.6 },
    ),
  ].join("");
  const svg = [
    `<rect class="f-sky" x="0" y="0" width="${W}" height="${H}"/>`,
    pencil(patches, "sky-deep", 1.6, 'opacity="0.85"'),
    path(glow, "glow"),
    path(glowDeep, "glow-deep", undefined, 'opacity="0.8"'),
    pencil(
      hatch(
        [
          [0, 160],
          [W, 160],
          [W, 210],
          [0, 210],
        ],
        r,
        { angle: -8, gap: 12, roughness: 0.6 },
      ),
      "glow-deep",
      1.6,
      'opacity="0.6"',
    ),
  ].join("");
  return { x: 0, y: 0, w: W, h: H, svg, className: "cl-base" };
}

// Long low light fanning out from the sun.
function beams(): Layer {
  const cx = 880;
  const cy = 157;
  const svg = [-172, -156, -141, -126]
    .map((a, i) => {
      const r0 = 110;
      const r1 = 720;
      const spread = 1.6 + (i % 2);
      const p = (rad: number, da: number): Pt => [
        cx + rad * Math.cos(((a + da) * Math.PI) / 180),
        cy + rad * Math.sin(((a + da) * Math.PI) / 180),
      ];
      const pts = [p(r0, -0.6), p(r1, -spread), p(r1, spread), p(r0, 0.6)];
      return `<path class="f-glow cl-beam-shimmer" d="M${pts.map(([x, y]) => `${n(x)} ${n(y)}`).join("L")}Z" opacity="0.4" style="--bs:${n(i * 0.9)}s"/>`;
    })
    .join("");
  return {
    x: 0,
    y: 0,
    w: W,
    h: H,
    svg,
    className: "cl-flat",
    enter: { delay: 1.8, kind: "fade" },
  };
}

// The low October sun, pulled down into a knit hat.
function sun(): Layer {
  const w = 230;
  const c = 115;
  const rays = Array.from({ length: 14 }, (_, i) => {
    const a = (i / 14) * Math.PI * 2 + 0.12;
    const r0 = 70;
    const r1 = i % 2 ? 88 : 98;
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
        rng(2100 + i),
        { taper: 0.45 },
      ),
      "sun-deep",
    );
  }).join("");
  const r = rng(2121);
  const disc = cutCircle(c, c, 62, r, 28);
  const hat: Pt[] = [
    [c - 64, c - 8],
    ...arcPoints(c, c - 10, 63, Math.PI * 1.02, Math.PI * 1.98, 14).map(
      ([x, y]) => [x, c - 10 - (c - 10 - y) * 0.92] as Pt,
    ),
    [c + 64, c - 8],
  ];
  const cuff: Pt[] = [
    [c - 67, c - 24],
    [c + 67, c - 24],
    [c + 66, c - 2],
    [c - 66, c - 2],
  ];
  const ribs = Array.from({ length: 15 }, (_, i) => {
    const x = c - 60 + i * 8.6;
    return `M${n(x)} ${n(c - 21)}L${n(x + 0.6)} ${n(c - 5)}`;
  }).join("");
  const stripes = [c - 40, c - 54]
    .map((y) =>
      line([c - 56, y], [c + 56, y], r, { roughness: 0.4, double: false }),
    )
    .join("");
  const pom = `<g class="cl-pompom" style="transform-origin:${c}px ${c - 70}px">${`<circle class="f-white" cx="${c}" cy="${c - 80}" r="14"/>`}${pencil(
    hatch(ring(c + 3, c - 78, 11, 11, 16), r, { angle: 40, gap: 3.5 }),
    "cloud-shade",
    1.8,
  )}${boil((rr) => pencil(ellipse(c, c - 80, 14, 14, rr, { double: false, roughness: 1.4 }), "ink", 1.2), 2122)}</g>`;
  const face = boil(
    (rr) =>
      [
        pencil(
          curve([
            [c - 32, c + 14],
            [c - 23, c + 6],
            [c - 13, c + 14],
          ]),
          "ink",
          3.2,
        ),
        pencil(
          curve([
            [c + 13, c + 14],
            [c + 23, c + 6],
            [c + 32, c + 14],
          ]),
          "ink",
          3.2,
        ),
        pencil(
          curve([
            [c - 14, c + 30],
            [c, c + 40],
            [c + 14, c + 30],
          ]),
          "ink",
          3,
        ),
        pencil(
          line([c - 2, c + 33], [c + 2, c + 33], rr, { double: false }),
          "ink",
          1,
        ),
      ].join(""),
    2123,
  );
  const svg = [
    group(rays, `class="cl-sun-rays" style="transform-origin:${c}px ${c}px"`),
    path(disc, "sun"),
    pencil(
      hatch(ring(c + 12, c + 16, 44, 40, 24), r, {
        angle: 35,
        gap: 5,
        roughness: 0.5,
      }),
      "sun-deep",
      2.2,
      'opacity="0.7"',
    ),
    boil(
      (rr) =>
        pencil(
          ellipse(c + 3, c + 2, 63, 63, rr, { roughness: 0.6 }),
          "sun-deep",
          1.6,
        ),
      2124,
    ),
    `<ellipse class="f-orange-soft" cx="${c - 38}" cy="${c + 26}" rx="10" ry="6" opacity="0.85"/><ellipse class="f-orange-soft" cx="${c + 38}" cy="${c + 26}" rx="10" ry="6" opacity="0.85"/>`,
    face,
    pom,
    path(cut(hat, r, 0.5), "stamp"),
    pencil(stripes, "white", 4, 'opacity="0.9"'),
    path(cut(cuff, r, 0.5), "fiber"),
    pencil(ribs, "cloud-shade", 1.6),
    boil(
      (rr) =>
        pencil(
          outline(cuff, rr, { roughness: 0.4, double: false }),
          "ink",
          1.2,
        ),
      2125,
    ),
  ].join("");
  return {
    x: 765,
    y: 42,
    w,
    h: w,
    svg,
    enter: { delay: 0.25, y: 30, rotate: 10, kind: "place" },
    loop: "cl-sink",
  };
}

// A palm with drooping fronds, for behind the glasshouse panes.
function palm(
  x: number,
  base: number,
  top: number,
  size: number,
  seed: number,
): string {
  const rr = rng(seed);
  const crown: Pt = [x + between(rr, -3, 3), top];
  const trunk = path(
    brush(
      [
        [x, base],
        [(x + crown[0]) / 2 + between(rr, -2, 2), (base + top) / 2],
        crown,
      ],
      size * 0.16,
      rr,
      { taper: 0.05, wobble: 0.1 },
    ),
    "trunk",
  );
  const rings = Array.from({ length: Math.floor((base - top) / 7) }, (_, i) => {
    const y = base - 5 - i * 7;
    const cx = x + (crown[0] - x) * ((base - y) / (base - top));
    return `M${n(cx - size * 0.07)} ${n(y)}l${n(size * 0.14)} -1.5`;
  }).join("");
  const fronds = [-172, -142, -112, -90, -68, -38, -8]
    .map((a, i) => {
      const len = size * between(rr, 0.8, 1.05);
      const rad = (a * Math.PI) / 180;
      const spine: Pt[] = [
        crown,
        [
          crown[0] + Math.cos(rad) * len * 0.5,
          crown[1] + Math.sin(rad) * len * 0.5 - size * 0.06,
        ],
        [
          crown[0] + Math.cos(rad) * len,
          crown[1] + Math.sin(rad) * len + size * 0.38,
        ],
      ];
      const leaflets = sample(spine, 9)
        .slice(2)
        .map(([px, py], k) => {
          const l = size * (0.16 - k * 0.012);
          return `M${n(px)} ${n(py)}l${n(-l * 0.5)} ${n(l)}M${n(px)} ${n(py)}l${n(l * 0.5)} ${n(l)}`;
        })
        .join("");
      return (
        pencil(leaflets, i % 2 ? "leaf-green" : "grass", 1.6) +
        path(
          brush(spine, size * 0.07, rr, { taper: 0.3, wobble: 0.2 }),
          "grass-dark",
        )
      );
    })
    .join("");
  return trunk + pencil(rings, "kraft-dark", 1) + fronds;
}

// The glasshouses of the Kaisaniemi Botanic Garden in a three-quarter view,
// receding up to the right: the barrel-vaulted Palm House between two long
// wings whose curved glass roofs sweep up to a crested flat top, and a small
// gabled house at either end. Palms and greenery show through the panes and
// the sun-facing glass catches the low light.
function palmHouse(): Layer {
  const r = rng(2201);
  const D: Pt = [34, -20];
  const G = 176;
  const P = (x: number, t: number, z: number): Pt => [
    x + D[0] * t,
    G - z + D[1] * t,
  ];
  const along = ([x, y]: Pt, t: number): Pt => [x + D[0] * t, y + D[1] * t];
  const deep = (pts: Pt[]) => pts.map((q) => along(q, 1));
  const bar = (a: Pt, b: Pt) =>
    line(a, b, r, { roughness: 0.2, double: false });
  const spaced = (from: number, to: number, gap: number) =>
    Array.from(
      { length: Math.max(0, Math.floor((to - from) / gap)) },
      (_, i) => from + gap / 2 + i * gap,
    );
  const warm = (pts: Pt[], gap: number) =>
    pencil(
      hatch(pts, r, { angle: 62, gap, roughness: 0.4 }),
      "glow-deep",
      2.2,
      'opacity="0.75"',
    );
  const lit = (pts: Pt[]) =>
    path(cut(pts, r, 0.4), "white", undefined, 'opacity="0.35"');
  const glass = (pts: Pt[]) => path(cut(pts, r, 0.4), "glass");
  const ink = (polys: Pt[][], open: Pt[][], seed: number) =>
    boil(
      (rr) =>
        pencil(
          [
            ...polys.map((pts) =>
              outline(pts, rr, { roughness: 0.35, double: false }),
            ),
            ...open.map((pts) =>
              outline(pts, rr, {
                roughness: 0.35,
                double: false,
                closed: false,
              }),
            ),
          ].join(""),
          "ink",
          1.2,
          'opacity="0.8"',
        ),
      seed,
    );

  const shrubs = [
    20, 40, 66, 90, 114, 132, 160, 188, 236, 262, 298, 326, 354, 382, 412, 434,
  ]
    .map((x, i) => {
      const sr = rng(2230 + i);
      const blob = ring(
        x,
        172,
        between(sr, 12, 17),
        between(sr, 8, 12),
        16,
      ).map(
        ([px, py]) =>
          [px + between(sr, -1.5, 1.5), py + between(sr, -1.5, 1.5)] as Pt,
      );
      return (
        path(torn(blob, sr, { amp: 1.4 }), i % 3 ? "grass" : "leaf-green") +
        pencil(
          hatch(blob.slice(8), sr, { angle: -40, gap: 4.5, roughness: 0.4 }),
          "grass-dark",
          1.2,
          'opacity="0.7"',
        )
      );
    })
    .join("");
  const plants = [
    palm(206, 176, 64, 60, 2241),
    palm(80, 176, 124, 30, 2242),
    palm(120, 176, 134, 22, 2243),
    palm(356, 176, 118, 34, 2244),
    palm(306, 176, 128, 26, 2245),
    leafShape(166, 150, 13, -30, "leaf-green", rng(2246)),
    leafShape(252, 146, 14, 34, "grass", rng(2247)),
    leafShape(334, 154, 12, 24, "leaf-green", rng(2248)),
    shrubs,
  ].join("");
  const planted = (faces: Pt[][], id: string) =>
    `<clipPath id="${id}"><path d="${faces.map((pts) => cut(pts, rng(2290), 0.1)).join("")}"/></clipPath><g clip-path="url(#${id})">${plants}</g>${faces.map((pts) => path(cut(pts, r, 0.4), "glass", undefined, 'opacity="0.3"')).join("")}`;

  // A small glasshouse with its gable to the front.
  const gabled = (x0: number, x1: number, eave: number, top: number) => {
    const xm = (x0 + x1) / 2;
    const ridge: Pt = [xm, top];
    const eaveL: Pt = [x0, eave];
    const eaveR: Pt = [x1, eave];
    const footR: Pt = [x1, G];
    const roofAt = (x: number) =>
      eave - (eave - top) * (1 - Math.abs(x - xm) / (xm - x0));
    const front: Pt[] = [[x0, G], eaveL, ridge, eaveR, footR];
    const leftSlope: Pt[] = [eaveL, ridge, along(ridge, 1), along(eaveL, 1)];
    const rightSlope: Pt[] = [ridge, eaveR, along(eaveR, 1), along(ridge, 1)];
    const side: Pt[] = [footR, eaveR, along(eaveR, 1), along(footR, 1)];
    // a steep left slope faces away and folds under the ridge
    const seesLeft = (xm - x0) * D[1] - (top - eave) * D[0] < 0;
    const frames = [
      ...spaced(x0, x1, 11).map((x) => bar([x, G], [x, roofAt(x)])),
      bar(eaveL, eaveR),
      bar([x0, (G + eave) / 2], [x1, (G + eave) / 2]),
      ...[0.25, 0.5, 0.75].flatMap((t) => [
        seesLeft ? bar(along(ridge, t), along(eaveL, t)) : "",
        bar(along(ridge, t), along(eaveR, t)),
        bar(along(footR, t), along(eaveR, t)),
      ]),
      bar(ridge, along(ridge, 1)),
    ].join("");
    const slopes = seesLeft ? [leftSlope, rightSlope] : [rightSlope];
    return {
      front,
      leftSlope: seesLeft ? leftSlope : null,
      rightSlope,
      side,
      frames,
      outlines: [...slopes, side],
      glass: [front, ...slopes, side],
    };
  };
  const gabledSvg = (h: ReturnType<typeof gabled>, id: string, seed: number) =>
    [
      h.leftSlope ? glass(h.leftSlope) + lit(h.leftSlope) : "",
      glass(h.rightSlope) + warm(h.rightSlope, 7),
      glass(h.side) + warm(h.side, 7),
      glass(h.front),
      planted([h.front], id),
      pencil(h.frames, "white", 1.5),
      ink(h.outlines, [h.front], seed),
    ].join("");

  // A long wing with its side to the front: upright glass walls and a roof
  // that curves up from the eaves to a flat, crested top.
  const curved = (x0: number, x1: number, wall: number, rise: number) => {
    const flat = 0.35;
    const back = 1 - flat;
    const z = (t: number) => {
      const u = Math.min(t, 1 - t);
      if (u >= flat) return wall + rise;
      const c = 1 - u / flat;
      return wall + rise * Math.sqrt(Math.max(0, 1 - c * c));
    };
    const profile = (x: number, a: number, b: number) =>
      Array.from({ length: 13 }, (_, i) => {
        const t = a + ((b - a) * i) / 12;
        return P(x, t, z(t));
      });
    const front: Pt[] = [
      P(x0, 0, 0),
      P(x0, 0, wall),
      P(x1, 0, wall),
      P(x1, 0, 0),
    ];
    const roof: Pt[] = [...profile(x1, 0, back), ...profile(x0, back, 0)];
    const end: Pt[] = [P(x1, 0, 0), ...profile(x1, 0, 1), P(x1, 1, 0)];
    const frames = [
      ...spaced(x0, x1, 12).map((x) => bar(P(x, 0, 0), P(x, 0, wall))),
      ...[wall / 2, wall].map((h) => bar(P(x0, 0, h), P(x1, 0, h))),
      ...spaced(x0, x1, 12).map((x) => curve(profile(x, 0, flat))),
      ...[0.12, 0.24, flat, back].map((t) =>
        bar(P(x0, t, z(t)), P(x1, t, z(t))),
      ),
      ...[0.25, 0.5, 0.75].map((t) => bar(P(x1, t, 0), P(x1, t, z(t)))),
      bar(P(x1, 0, wall / 2), P(x1, 1, wall / 2)),
    ].join("");
    const crest = Array.from(
      { length: Math.floor((x1 - x0) / 5) + 1 },
      (_, i) => {
        const [cx, cy] = P(x0 + i * 5, flat, wall + rise);
        return `M${n(cx)} ${n(cy)}l0 -4`;
      },
    ).join("");
    const [ax, ay] = P(x0, flat, wall + rise);
    const [bx, by] = P(x1, flat, wall + rise);
    return {
      front,
      roof,
      end,
      frames,
      crest: `${crest}M${n(ax)} ${n(ay - 4)}L${n(bx)} ${n(by - 4)}`,
      glass: [front, roof, end],
    };
  };
  const curvedSvg = (w: ReturnType<typeof curved>, id: string, seed: number) =>
    [
      glass(w.end) + warm(w.end, 7),
      glass(w.roof) + lit(w.roof),
      glass(w.front),
      planted([w.front, w.roof], id),
      pencil(w.frames, "white", 1.5),
      pencil(w.crest, "white", 1.4),
      ink([w.end], [w.roof, w.front], seed),
    ].join("");

  const arch = arcPoints(210, 92, 70, Math.PI, 2 * Math.PI, 18).map(
    ([x, y]) => [x, 92 - (92 - y) * 0.95] as Pt,
  );
  const pavilion = {
    front: [[140, 176], ...arch, [280, 176]] as Pt[],
    vault: [...arch, ...deep(arch).reverse()],
    side: [
      [280, 176],
      [280, 92],
      along([280, 92], 1),
      along([280, 176], 1),
    ] as Pt[],
    vaultFrames: [
      ...arch.filter((_, i) => i % 3 === 0).map((q) => bar(q, along(q, 1))),
      curve(arch.map((q) => along(q, 0.5))),
      ...[0.33, 0.66].map((t) =>
        bar(along([280, 176], t), along([280, 92], t)),
      ),
      ...[116, 140].map((y) => bar([280, y], along([280, y], 1))),
    ].join(""),
    frames: [
      ...Array.from({ length: 10 }, (_, i) => {
        const x = 147 + i * 14;
        return bar(
          [x, 176],
          [
            x,
            92 - 70 * 0.95 * Math.sqrt(Math.max(0, 1 - ((x - 210) / 70) ** 2)),
          ],
        );
      }),
      ...[92, 116, 140].map((y) => bar([140, y], [280, y])),
    ].join(""),
  };

  const farLeft = gabled(4, 52, 146, 124);
  const leftWing = curved(52, 140, 44, 24);
  const rightWing = curved(280, 390, 46, 26);
  const farRight = gabled(390, 446, 140, 108);
  const glassFaces = [
    ...farLeft.glass,
    ...leftWing.glass,
    pavilion.vault,
    pavilion.side,
    pavilion.front,
    ...rightWing.glass,
    ...farRight.glass,
  ].map((pts) => cut(pts, r, 0.4));
  const plinthFront: Pt[] = [
    [2, 176],
    [448, 176],
    [448, 198],
    [2, 198],
  ];
  const plinthSide: Pt[] = [
    [448, 176],
    along([448, 176], 1),
    along([448, 198], 1),
    [448, 198],
  ];
  const svg = [
    `<clipPath id="cl-palm-glass"><path d="${glassFaces.join("")}"/></clipPath>`,
    path(cut(plinthSide, r, 0.3), "wall-shade"),
    gabledSvg(farLeft, "cl-palm-a", 2202),
    curvedSvg(leftWing, "cl-palm-b", 2204),
    glass(pavilion.vault),
    lit(pavilion.vault),
    glass(pavilion.side) + warm(pavilion.side, 7),
    pencil(pavilion.vaultFrames, "white", 1.6),
    boil(
      (rr) =>
        pencil(
          curve(wobble(deep(arch), rr, 0.6)) +
            outline(pavilion.side, rr, { roughness: 0.35, double: false }),
          "ink",
          1.3,
          'opacity="0.8"',
        ),
      2206,
    ),
    glass(pavilion.front),
    planted([pavilion.front], "cl-palm-c"),
    pencil(pavilion.frames, "white", 1.6),
    boil(
      (rr) =>
        pencil(
          outline(pavilion.front.slice(0, 2), rr, {
            double: false,
            closed: false,
          }) +
            outline(pavilion.front.slice(-2), rr, {
              double: false,
              closed: false,
            }) +
            curve(wobble(arch, rr, 0.6)),
          "ink",
          1.3,
          'opacity="0.8"',
        ),
      2207,
    ),
    curvedSvg(rightWing, "cl-palm-d", 2208),
    gabledSvg(farRight, "cl-palm-e", 2210),
    `<g clip-path="url(#cl-palm-glass)"><g class="cl-glint"><path class="f-white" d="M-40 -40L-4 -40L-64 220L-100 220Z" opacity="0.6"/><path class="f-white" d="M8 -40L18 -40L-42 220L-52 220Z" opacity="0.5"/></g></g>`,
    path(cut(plinthFront, r, 0.5), "wall"),
    pencil(
      hatch(
        [
          [4, 186],
          [446, 186],
          [446, 196],
          [4, 196],
        ],
        r,
        { angle: 70, gap: 5, roughness: 0.3 },
      ),
      "wall-shade",
      1.6,
    ),
    path(cutRect(196, 146, 28, 30, r, 0.3), "granite-dark"),
    pencil("M210 147L210 175", "granite", 1.2),
  ].join("");
  return {
    x: 265,
    y: 146,
    w: 490,
    h: 230,
    svg: `<g transform="translate(0 30)">${svg}</g>`,
    enter: { delay: 0.8, y: 30, kind: "grow" },
  };
}

// A herring gull in a red scarf cycling along the park path now and then.
function cyclist(): Layer {
  const r = rng(2321);
  const top = 310;
  const home = 420;
  const bx = home - 38;
  const by = pathMiddle(home) - top - 67;
  const hub = (cx: number) => [
    `<circle class="s-ink" cx="${cx}" cy="54" r="13" fill="none" stroke-width="2.4"/>`,
    `<g class="cl-wheel" style="transform-origin:${cx}px 54px">${pencil(
      [0, 45, 90, 135]
        .map((a) => {
          const c = Math.cos((a * Math.PI) / 180) * 12;
          const s = Math.sin((a * Math.PI) / 180) * 12;
          return `M${n(cx - c)} ${n(54 - s)}L${n(cx + c)} ${n(54 + s)}`;
        })
        .join(""),
      "pencil",
      0.9,
    )}</g>`,
    `<circle class="f-ink" cx="${cx}" cy="54" r="1.8"/>`,
  ];
  const bb: Pt = [36, 55];
  const hip: Pt = [31, 29];
  const knee = (pedal: Pt): Pt => {
    const d = Math.min(31.9, Math.hypot(pedal[0] - hip[0], pedal[1] - hip[1]));
    const a = Math.atan2(pedal[1] - hip[1], pedal[0] - hip[0]);
    const b = a - Math.acos(d / 32);
    return [hip[0] + 16 * Math.cos(b), hip[1] + 16 * Math.sin(b)];
  };
  const pedalling = [0, 1, 2]
    .map((f) => {
      const a = (f * 120 * Math.PI) / 180;
      const near: Pt = [bb[0] + 6.5 * Math.cos(a), bb[1] + 6.5 * Math.sin(a)];
      const far: Pt = [bb[0] - 6.5 * Math.cos(a), bb[1] - 6.5 * Math.sin(a)];
      const leg = (pedal: Pt, tone: string) =>
        pencil(curve([hip, knee(pedal), pedal]), tone, 2.6) +
        path(
          `M${n(pedal[0] - 2)} ${n(pedal[1])}L${n(pedal[0] + 5)} ${n(pedal[1] - 1)}L${n(pedal[0] + 4)} ${n(pedal[1] + 2)}Z`,
          tone,
        );
      return `<g class="cl-cy cl-cy${f}" style="--cycle:0.6s">${[
        leg(far, "sun-deep"),
        pencil(
          `M${n(far[0])} ${n(far[1])}L${n(near[0])} ${n(near[1])}`,
          "ink",
          2,
        ),
        `<circle class="f-granite-dark" cx="${bb[0]}" cy="${bb[1]}" r="3.4"/>`,
        leg(near, "orange"),
      ].join("")}</g>`;
    })
    .join("");
  const frame = pencil(
    "M18 54L36 55L31 35L18 54M31 35L51 36L36 55M51 36L58 54M51 36L53 29M49 29L57 30",
    "stamp",
    2.4,
  );
  const scarfTails = [0, 1, 2]
    .map((f) => {
      const wave = [0, 2.5, -2][f];
      return `<g class="cl-cy cl-cy${f}">${path(
        cut(
          [
            [45, 15],
            [34, 12 + wave],
            [24, 15 - wave],
            [26, 19 - wave],
            [35, 17 + wave],
            [45, 19],
          ],
          r,
          0.3,
        ),
        "scarf",
      )}</g>`;
    })
    .join("");
  const gull = [
    scarfTails,
    path(
      cut(
        [
          [18, 15],
          [27, 17],
          [28, 24],
          [20, 22],
        ],
        r,
        0.3,
      ),
      "white",
    ),
    `<ellipse class="f-white" cx="37" cy="21" rx="13.5" ry="9" transform="rotate(-16 37 21)"/>`,
    pencil(
      ellipse(37, 21, 13.5, 9, r, { roughness: 0.4, double: false }),
      "ink",
      1,
      'opacity="0.7" transform="rotate(-16 37 21)"',
    ),
    path(
      cut(
        [
          [25, 17],
          [40, 16],
          [53, 27],
          [50, 30],
          [38, 25],
          [27, 23],
        ],
        r,
        0.3,
      ),
      "cloud-shade",
    ),
    path(
      cut(
        [
          [48, 26],
          [53, 27],
          [50, 30],
          [47, 29],
        ],
        r,
        0.2,
      ),
      "ink",
    ),
    pencil("M26 20L39 19", "slate", 1, 'opacity="0.7"'),
    `<circle class="f-white" cx="51" cy="8" r="7.5"/>`,
    pencil(
      ellipse(51, 8, 7.5, 7.5, r, { roughness: 0.4, double: false }),
      "ink",
      1,
      'opacity="0.7"',
    ),
    path("M57 6.5L66 9L57 11Z", "sun"),
    `<circle class="f-stamp" cx="62.6" cy="9.6" r="1"/>`,
    `<circle class="f-ink" cx="53.5" cy="6" r="1.2"/>`,
    path(cutRect(43, 14, 11, 5, r, 0.3), "scarf"),
  ].join("");
  const bike = [
    ...hub(18),
    ...hub(58),
    frame,
    pencil("M25 32L35 32", "ink", 3.2),
    pedalling,
    gull,
  ].join("");
  const keys = Array.from({ length: 29 }, (_, i) => {
    const x = -110 + i * 50;
    return `${n((i / 28) * 58)}%{transform:translate(${n(x - home)}px,${n(pathMiddle(x) - pathMiddle(home))}px)}`;
  }).join("");
  return {
    x: 0,
    y: top,
    w: W,
    h: 150,
    svg: `<style>@keyframes cl-ride{${keys}100%{transform:translate(${n(1290 - home)}px,${n(pathMiddle(1290) - pathMiddle(home))}px)}}</style><g class="cl-ride"><g transform="translate(${n(bx)} ${n(by)})">${bike}</g></g>`,
    className: "cl-flat",
  };
}

function wire(): Layer {
  const r = rng(2311);
  const svg = [
    pencil(
      line([-10, 354], [W + 10, 354], r, { roughness: 0.3, double: false }),
      "granite-dark",
      1.6,
    ),
    path(
      tornRect(-10, 354, W + 20, 18, r, {
        amp: 1.2,
        torn: [false, false, false, false],
      }),
      "granite",
    ),
  ].join("");
  return {
    x: 0,
    y: 0,
    w: W,
    h: H,
    svg,
    className: "cl-flat",
    enter: { delay: 1, kind: "fade" },
  };
}

function crown(
  cx: number,
  cy: number,
  rx: number,
  ry: number,
  tone: string,
  hatchTone: string,
  seed: number,
): string {
  const rr = rng(seed);
  const pts = ring(cx, cy, rx, ry, 30, rr() * 6).map(
    ([x, y]) => [x + between(rr, -4, 4), y + between(rr, -4, 4)] as Pt,
  );
  const speckles = Array.from({ length: 18 }, () => {
    const a = rr() * 6.28;
    const k = Math.sqrt(rr()) * 0.85;
    const x = cx + rx * k * Math.cos(a);
    const y = cy + ry * k * Math.sin(a);
    return leafShape(
      x,
      y,
      between(rr, 3, 4.6),
      between(rr, -80, 80),
      LEAF_TONES[Math.floor(rr() * LEAF_TONES.length)],
      rr,
    );
  }).join("");
  return [
    path(
      torn(
        pts.map(([x, y]) => [cx + (x - cx) * 1.04, cy + (y - cy) * 1.04] as Pt),
        rr,
        { amp: 2.4 },
      ),
      "fiber",
    ),
    path(torn(pts, rr, { amp: 2.4 }), tone),
    pencil(
      hatch(
        pts.filter((_, i) => i < 16),
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
  ].join("");
}

function tree(
  cx: number,
  base: number,
  crowns: [number, number, number, number, string, string][],
  trunkTone: string,
  seed: number,
  birch = false,
): string {
  const rr = rng(seed);
  const top = Math.min(...crowns.map(([, y, , ry]) => y + ry * 0.4));
  const trunk = path(
    cut(
      [
        [cx - 7, base],
        [cx - 4, top],
        [cx + 4, top],
        [cx + 7, base],
      ],
      rr,
      0.4,
    ),
    trunkTone,
  );
  const marks = birch
    ? pencil(
        Array.from({ length: 6 }, (_, i) => {
          const y = top + 14 + i * 16 + between(rr, -3, 3);
          return `M${n(cx - 5)} ${n(y)}L${n(cx + between(rr, -1, 2))} ${n(y + 1.5)}`;
        }).join(""),
        "ink",
        2.2,
      )
    : "";
  return group(
    [
      trunk,
      marks,
      ...crowns.map(([x, y, rx, ry, tone, shade], i) =>
        crown(x, y, rx, ry, tone, shade, seed * 10 + i),
      ),
    ].join(""),
    `class="cl-sway" style="transform-origin:${cx}px ${base}px;--sway:${n(between(rr, 2.4, 3.6))}s"`,
  );
}

function bareTree(cx: number, base: number, seed: number): string {
  const r = rng(seed);
  const trunk: Pt[] = [
    [cx, base],
    [cx - 2, base - 70],
    [cx + 4, base - 140],
    [cx + 2, base - 190],
  ];
  const limbs: Pt[][] = [
    [
      [cx - 1, base - 92],
      [cx - 34, base - 134],
      [cx - 52, base - 170],
    ],
    [
      [cx + 3, base - 120],
      [cx + 38, base - 160],
      [cx + 62, base - 176],
    ],
    [
      [cx + 3, base - 160],
      [cx - 18, base - 196],
      [cx - 26, base - 218],
    ],
    [
      [cx - 30, base - 128],
      [cx - 60, base - 140],
    ],
    [
      [cx + 30, base - 152],
      [cx + 44, base - 196],
    ],
  ];
  const tips: Pt[] = [
    [cx - 52, base - 170],
    [cx + 62, base - 176],
    [cx - 26, base - 218],
    [cx - 60, base - 140],
    [cx + 44, base - 196],
    [cx + 2, base - 190],
  ];
  const leaves = tips
    .flatMap(([x, y], i) =>
      Array.from({ length: 2 }, (_, k) =>
        leafShape(
          x + between(r, -8, 8),
          y + between(r, -4, 10),
          between(r, 4.5, 6.5),
          between(r, -60, 60),
          LEAF_TONES[(i + k) % LEAF_TONES.length],
          r,
        ),
      ),
    )
    .join("");
  return group(
    [
      path(brush(trunk, 14, r, { taper: 0.08, wobble: 0.1 }), "trunk"),
      ...limbs.map((l) =>
        path(brush(l, 5, r, { taper: 0.5, wobble: 0.2 }), "trunk"),
      ),
      leaves,
    ].join(""),
    `class="cl-sway" style="transform-origin:${cx}px ${base}px;--sway:3.8s"`,
  );
}

function treesLeft(): Layer {
  const svg = [
    tree(70, 252, [[66, 92, 44, 66, "birch", "sun-deep"]], "bark", 241, true),
    tree(
      180,
      252,
      [
        [178, 100, 74, 70, "maple", "rowan"],
        [140, 128, 40, 36, "rowan", "maple"],
      ],
      "trunk",
      242,
    ),
    tree(284, 252, [[282, 142, 40, 46, "birch", "sun-deep"]], "trunk", 243),
  ].join("");
  return {
    x: -20,
    y: 116,
    w: 340,
    h: 256,
    svg,
    enter: { delay: 1.1, y: 30, kind: "grow" },
  };
}

function treesRight(): Layer {
  const svg = [
    tree(
      96,
      266,
      [
        [94, 104, 80, 82, "maple", "rowan"],
        [140, 150, 44, 40, "sun", "sun-deep"],
      ],
      "trunk",
      251,
    ),
    bareTree(218, 266, 252),
  ].join("");
  return {
    x: 930,
    y: 104,
    w: 290,
    h: 268,
    svg,
    enter: { delay: 1.2, y: 30, kind: "grow" },
  };
}

function lawn(): Layer {
  const r = rng(2401);
  const grass = tornRect(-10, 8, W + 20, 170, r, {
    amp: 3.5,
    step: 5,
    torn: [true, false, false, false],
  });
  const walk = (y: number) =>
    Array.from({ length: 13 }, (_, i) => [-10 + i * 102, y + walkY(i)] as Pt);
  const top = walk(0);
  const bottom = walk(30).reverse();
  const leaves = Array.from({ length: 90 }, () => {
    const x = between(r, 0, W);
    const y = between(r, 20, 156);
    return leafShape(
      x,
      y,
      between(r, 3.4, 5.4),
      between(r, -90, 90),
      LEAF_TONES[Math.floor(r() * LEAF_TONES.length)],
      r,
    );
  }).join("");
  const svg = [
    path(grass, "grass"),
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
    path(cut([...top, ...bottom], r, 0.8), "path"),
    pencil(
      hatch([...top, ...bottom], r, { angle: 15, gap: 7, roughness: 0.6 }),
      "kraft-dark",
      1.2,
      'opacity="0.45"',
    ),
    leaves,
  ].join("");
  return {
    x: 0,
    y: LAWN_Y,
    w: W,
    h: 170,
    svg,
    enter: { delay: 0.6, y: 50, kind: "grow" },
  };
}

// A park worker rakes a leaf pile; three takes of the stroke.
function raker(): Layer {
  const r = rng(2501);
  const pile: Pt[] = [
    [20, 98],
    [30, 82],
    [52, 66],
    [80, 58],
    [108, 62],
    [132, 74],
    [148, 90],
    [154, 98],
  ];
  const pileLeaves = Array.from({ length: 46 }, () => {
    const t = r();
    const x = 26 + t * 124;
    const topY = 98 - Math.sin(t * Math.PI) * 38;
    const y = between(r, topY + 2, 98);
    return leafShape(
      x,
      y,
      between(r, 4, 6),
      between(r, -90, 90),
      LEAF_TONES[Math.floor(r() * LEAF_TONES.length)],
      r,
    );
  }).join("");
  const mound = [
    path(cut(pile, r, 0.8), "maple"),
    pencil(
      hatch(pile, r, { angle: -30, gap: 6, roughness: 0.6 }),
      "rowan",
      1.6,
      'opacity="0.6"',
    ),
    pileLeaves,
    boil(
      (rr) => pencil(curve(wobble(pile, rr, 0.8)), "ink", 1.1, 'opacity="0.6"'),
      2502,
    ),
  ].join("");
  const frames = [0, 1, 2]
    .map((f) => {
      const fr = rng(2510 + f);
      const reach = [0, 10, 20][f];
      const head: Pt = [150 + reach, 98];
      const hands: [Pt, Pt] = [
        [214 - reach * 0.3, 52 + reach * 0.15],
        [230 - reach * 0.2, 40],
      ];
      const handle = `M${n(head[0])} ${n(head[1] - 4)}L${n(hands[1][0] + 8)} ${n(hands[1][1] - 6)}`;
      const tines = Array.from({ length: 6 }, (_, i) => {
        const x = head[0] - 12 + i * 4.6;
        return `M${n(x)} ${n(head[1] - 6)}L${n(x - 2)} ${n(head[1] + 1)}`;
      }).join("");
      const lean = [-4, -8, -12][f];
      return `<g class="cl-cy cl-cy${f}" style="--cycle:1.5s">${[
        pencil(handle, "trunk", 3),
        pencil(
          `M${n(head[0] - 13)} ${n(head[1] - 6)}L${n(head[0] + 13)} ${n(head[1] - 6)}`,
          "ink",
          2.4,
        ),
        pencil(tines, "ink", 1.4),
        `<g transform="rotate(${lean} 240 102)">${[
          pencil("M232 66L226 102M246 66L250 102", "ink", 4.6),
          `<ellipse class="f-ink" cx="224" cy="102" rx="6" ry="3"/><ellipse class="f-ink" cx="252" cy="102" rx="6" ry="3"/>`,
          path(
            cut(
              [
                [226, 30],
                [252, 30],
                [256, 70],
                [222, 70],
              ],
              fr,
              0.5,
            ),
            "blue-soft",
          ),
          path(
            cut(
              [
                [228, 32],
                [250, 32],
                [253, 62],
                [225, 62],
              ],
              fr,
              0.4,
            ),
            "orange",
          ),
          pencil("M226 50L252 50", "white", 2.6, 'opacity="0.9"'),
          `<circle class="f-skin" cx="238" cy="18" r="10"/>`,
          path(
            cut(
              [
                [227, 16],
                ...arcPoints(238, 15, 11.5, Math.PI, 2 * Math.PI, 8),
                [249, 16],
              ],
              fr,
              0.3,
            ),
            "blue",
          ),
          `<circle class="f-white" cx="238" cy="3" r="3.4"/>`,
          `<circle class="f-ink" cx="232" cy="19" r="1.2"/>`,
          `<ellipse class="f-orange-soft" cx="232" cy="24" rx="2.6" ry="1.6" opacity="0.8"/>`,
        ].join("")}</g>`,
        pencil(curve([[236, 36], [226, 46], hands[0]]), "blue-soft", 5),
        pencil(curve([[244, 34], [240, 36], hands[1]]), "blue-soft", 5),
        `<circle class="f-skin" cx="${n(hands[0][0])}" cy="${n(hands[0][1])}" r="3"/><circle class="f-skin" cx="${n(hands[1][0])}" cy="${n(hands[1][1])}" r="3"/>`,
      ].join("")}</g>`;
    })
    .join("");
  return {
    x: 470,
    y: 310,
    w: 270,
    h: 122,
    svg: `<g transform="translate(0 12)">${mound}${frames}</g>`,
    enter: { delay: 1.45, x: 20, kind: "place" },
  };
}

// A bench under the birches with a reader and a steaming cup.
function bench(): Layer {
  const r = rng(2601);
  const slats = [42, 50, 58]
    .map((y) => path(cutRect(10, y, 140, 5.5, r, 0.4), "kraft"))
    .join("");
  const back = [18, 27]
    .map((y) => path(cutRect(10, y, 140, 5.5, r, 0.4), "kraft"))
    .join("");
  const legs = pencil(
    "M18 62L14 86M142 62L146 86M20 30L20 62M140 30L140 62",
    "ink",
    3,
  );
  const reader = [
    pencil(
      curve([
        [96, 58],
        [116, 60],
        [120, 84],
      ]),
      "ink",
      4.6,
    ),
    pencil(
      curve([
        [92, 58],
        [108, 64],
        [110, 86],
      ]),
      "ink",
      4.6,
    ),
    path(
      cut(
        [
          [78, 22],
          [102, 22],
          [104, 58],
          [76, 58],
        ],
        r,
        0.5,
      ),
      "blue-soft",
    ),
    pencil(
      outline(
        [
          [78, 22],
          [102, 22],
          [104, 58],
          [76, 58],
        ],
        r,
        { roughness: 0.4, double: false },
      ),
      "ink",
      1.1,
    ),
    `<circle class="f-skin" cx="90" cy="10" r="9.5"/>`,
    path(
      cut(
        [[80, 8], ...arcPoints(90, 7, 11, Math.PI, 2 * Math.PI, 8), [101, 8]],
        r,
        0.3,
      ),
      "trunk",
    ),
    `<circle class="f-ink" cx="95" cy="11" r="1.1"/>`,
    path(cutRect(98, 30, 26, 20, r, 0.4), "paper"),
    pencil("M101 35L120 35M101 39L118 39M101 43L121 43", "pencil", 0.9),
    pencil(
      curve([
        [96, 30],
        [104, 40],
        [100, 44],
      ]),
      "blue-soft",
      5,
    ),
    path(
      cut(
        [
          [40, 40],
          [50, 40],
          [48, 54],
          [42, 54],
        ],
        r,
        0.3,
      ),
      "white",
    ),
    path(cutRect(40, 45, 10, 4, r, 0.2), "kraft"),
    ...[0, 1, 2].map(
      (i) =>
        `<g class="cl-steam" style="--std:${n(i * 0.8)}s;transform-origin:45px 38px">${pencil(
          curve([
            [43 + i * 2, 36],
            [40 + i * 2, 28],
            [46 + i * 2, 20],
            [42 + i * 2, 12],
          ]),
          "white",
          1.8,
        )}</g>`,
    ),
  ].join("");
  return {
    x: 168,
    y: 286,
    w: 160,
    h: 116,
    svg: `<g transform="translate(0 26)">${[back, legs, slats, reader].join("")}</g>`,
    enter: { delay: 1.35, y: 30, kind: "drop" },
  };
}

// Every few seconds a gust lifts leaves off the pile and the trees.
function flurry(): Layer[] {
  const r = rng(2701);
  const starts: [number, number][] = [
    ...Array.from(
      { length: 7 },
      () => [between(r, 500, 620), between(r, 382, 412)] as [number, number],
    ),
    ...Array.from(
      { length: 5 },
      () => [between(r, 940, 1080), between(r, 150, 250)] as [number, number],
    ),
    ...Array.from(
      { length: 3 },
      () => [between(r, 60, 240), between(r, 160, 240)] as [number, number],
    ),
  ];
  return starts.map(([x, y], i) => ({
    x,
    y,
    w: 22,
    h: 22,
    svg: leafShape(
      11,
      11,
      between(r, 6, 8),
      between(r, -40, 40),
      LEAF_TONES[i % LEAF_TONES.length],
      r,
    ),
    className: "cl-flurry",
    vars: {
      fx: `${n(between(r, 700, 1900))}%`,
      fy: `${n(between(r, -420, 120))}%`,
      fm: `${n(between(r, -260, -60))}%`,
      fd: `${n(3 + between(r, 0, 0.7) + (i >= 7 ? 0.35 : 0))}s`,
      spin: `${n(between(r, 0.7, 1.3))}s`,
    },
  }));
}

// A skein of geese heading south.
function flock(): Layer {
  const birds: Pt[] = [
    [120, 34],
    [104, 26],
    [88, 18],
    [72, 10],
    [104, 42],
    [88, 50],
    [70, 58],
  ];
  const frames = [-5, -1, 3]
    .map((tip, f) => {
      const d = birds
        .map(
          ([x, y]) =>
            `M${x - 7} ${y + tip}Q${x - 3} ${y - 4} ${x} ${y}Q${x + 3} ${y - 4} ${x + 7} ${y + tip}`,
        )
        .join("");
      return `<g class="cl-cy cl-cy${f}" style="--cycle:0.7s">${pencil(d, "ink", 1.7)}</g>`;
    })
    .join("");
  return {
    x: 330,
    y: 24,
    w: 140,
    h: 70,
    svg: frames,
    loop: "cl-flock",
    enter: { delay: 2, kind: "fade" },
  };
}

function gusts(): Layer {
  const lines: Pt[][] = [
    [
      [340, 115],
      [420, 103],
      [500, 113],
      [600, 97],
    ],
    [
      [780, 500],
      [850, 490],
      [920, 498],
      [1000, 486],
    ],
    [
      [1010, 81],
      [1070, 71],
      [1130, 79],
      [1190, 69],
    ],
  ];
  const svg = lines
    .map((pts, i) =>
      pencil(
        curve(pts),
        "pencil",
        1.8,
        `pathLength="1" style="--gd:${n(2.9 + i * 0.18)}s"`,
        "cl-gust-long",
      ),
    )
    .join("");
  return { x: 0, y: 0, w: W, h: H, svg, className: "cl-flat" };
}

export function goldenParkScene(title: string, notes: [string, string]): Scene {
  const layers: Layer[] = [
    sky(),
    ...at(0.04, beams(), flock()),
    ...at(0.06, sun()),
    ...at(
      0.12,
      {
        x: 0,
        y: 198,
        w: W,
        h: 190,
        svg: skyline(2801, 0, 0, W, 190, 70, 140, "block", "block-dark"),
        enter: { delay: 0.5, y: 40, kind: "grow" },
      },
      {
        x: 0,
        y: 262,
        w: W,
        h: 126,
        svg: skyline(2802, 0, 0, W, 126, 46, 96, "block-warm", "kraft-dark"),
        enter: { delay: 0.6, y: 40, kind: "grow" },
      },
    ),
    ...at(0.2, palmHouse()),
    ...at(0.24, {
      x: 752,
      y: 294,
      w: 70,
      h: 64,
      svg: stevensonScreen(0, 0, 0.72, 2901),
      enter: { delay: 1.25, y: -50, kind: "drop" },
    }),
    ...at(0.26, wire()),
    ...at(0.32, treesLeft(), treesRight()),
    ...at(0.4, lawn(), bench(), cyclist(), raker()),
    gusts(),
    ...flurry(),
    ...at(0.46, { ...scrap(title), x: 440, y: 448 }),
    ...at(
      0.18,
      note(
        640,
        20,
        120,
        80,
        notes[0],
        [
          [80, 30],
          [116, 28],
          [152, 50],
        ],
        2911,
        1.9,
        -5,
      ),
      note(
        616,
        212,
        130,
        70,
        notes[1],
        [
          [70, 44],
          [116, 50],
          [156, 80],
        ],
        2912,
        2.1,
        -3,
      ),
    ),
  ];
  return { w: W, h: H, layers };
}
