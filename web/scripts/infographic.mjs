import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { Resvg } from "@resvg/resvg-js";
import sharp from "sharp";
import { runnerImport } from "vite";

const path = (p) => fileURLToPath(new URL(`../${p}`, import.meta.url));
const load = async (p) =>
  (await runnerImport(path(p), { configFile: false, logLevel: "error" }))
    .module;

const { paletteLiteralCss } = await load("src/lib/collage/palette.ts");
const { heroScene } = await load("src/lib/collage/scenes/hero.ts");
const { goldenParkScene } = await load("src/lib/collage/scenes/goldenPark.ts");

const W = 1200;
const M = 80;
const PW = W - 2 * M;

const C = {
  paper: "#fbf6ec",
  card: "#fffdf8",
  ink: "#25303c",
  pencil: "#5d6b79",
  faint: "#8a96a3",
  grid: "#e6dfd0",
  night: "#17212b",
  teal: "#2dd4bf",
  warm: "#ea580c",
  warmSoft: "#f7a26b",
  warmWash: "#fde3cf",
  cool: "#2563eb",
  coolSoft: "#93b4f3",
  coolWash: "#dbe6fb",
  rain: "#4c7fbf",
  sun: "#f6b93b",
  sunDeep: "#ec9a1e",
  sunWash: "#fbecc8",
};

const SANS = "'Helvetica Neue', -apple-system, 'Segoe UI', Roboto, sans-serif";
const BOLD = "Inter";
const HAND = "Caveat";

function xml(s) {
  return String(s)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function t(x, y, s, o = {}) {
  const {
    size = 20,
    fill = C.ink,
    anchor = "start",
    family = SANS,
    weight = 400,
    spacing,
  } = o;
  const extra = spacing != null ? `letter-spacing="${spacing}"` : "";
  return `<text x="${x}" y="${y}" font-family="${family}" font-weight="${weight}" font-size="${size}" fill="${fill}" text-anchor="${anchor}" ${extra}>${xml(s)}</text>`;
}

const bold = (x, y, s, size, fill = C.ink, o = {}) =>
  t(x, y, s, { family: BOLD, weight: 700, size, fill, ...o });
const hline = (x1, x2, y, stroke, width = 1, extra = "") =>
  `<line x1="${x1}" x2="${x2}" y1="${y}" y2="${y}" stroke="${stroke}" stroke-width="${width}" ${extra}/>`;
const card = (x, y, w, h) =>
  `<rect x="${x}" y="${y}" width="${w}" height="${h}" rx="14" fill="${C.card}" stroke="${C.grid}" stroke-width="1.5"/>`;
const fmt = (v, d = 1) => v.toFixed(d).replace("-", "−");
const signed = (v, d = 1) => (v >= 0 ? "+" : "−") + Math.abs(v).toFixed(d);

const hidden = [
  ".cl-boil > .cl-f1",
  ".cl-boil > .cl-f2",
  ".cl-cy1",
  ".cl-cy2",
  ".cl-fr1",
  ".cl-fr2",
  ".cl-fr3",
  ".cl-face-awake",
];
const style = `${paletteLiteralCss(1)}.cl-hand{font-family:Caveat}${hidden.join(",")}{display:none}`;
const grain = (
  await sharp(path("src/assets/collage/grain.webp")).png().toBuffer()
).toString("base64");
const logo = readFileSync(path("public/wby-weather.svg")).toString("base64");

function collage(scene) {
  const layers = scene.layers.map((layer, i) => {
    const classes = (layer.className ?? "").split(" ");
    if (["cl-leaf", "cl-walker", "cl-flurry"].some((c) => classes.includes(c)))
      return "";
    const rain = classes.includes("cl-rain");
    const flat =
      rain || classes.includes("cl-base") || classes.includes("cl-flat");
    const { x, y, w, h } = layer;
    const svg = `<svg x="${x}" y="${y}" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}" preserveAspectRatio="none" overflow="${rain ? "hidden" : "visible"}">${layer.svg}</svg>`;
    if (rain) {
      return `<mask id="rain-${i}" maskUnits="userSpaceOnUse" x="${x}" y="${y}" width="${w}" height="${h}"><rect x="${x}" y="${y}" width="${w}" height="${h}" fill="url(#rain-fade)"/></mask><g mask="url(#rain-${i})">${svg}</g>`;
    }
    return flat ? svg : `<g filter="url(#shadow)">${svg}</g>`;
  });
  return `<svg width="${scene.w}" height="${scene.h}" viewBox="0 0 ${scene.w} ${scene.h}" overflow="hidden"><g style="isolation:isolate">${layers.join("")}<rect width="${scene.w}" height="${scene.h}" fill="url(#grain)" opacity="0.5" style="mix-blend-mode:soft-light"/></g></svg>`;
}

let out = [];
let y = 0;

function notes(list, sx, sy) {
  for (const n of list) {
    const x = sx(n.at);
    if (n.line) {
      out.push(
        `<line x1="${x}" x2="${x}" y1="${sy(n.line[0])}" y2="${sy(n.line[1])}" stroke="${C.pencil}" stroke-width="1.5"/>`,
      );
    }
    out.push(
      t(x + (n.dx ?? 0), sy(n.v) + (n.dy ?? 0), n.text, {
        family: HAND,
        weight: 700,
        size: n.size ?? 26,
        fill: n.color ?? C.pencil,
        anchor: n.anchor ?? "start",
      }),
    );
  }
}

function header(scene, title, subtitle) {
  out.push(collage(scene));
  y = scene.h;
  const band = 190;
  out.push(`<rect y="${y}" width="${W}" height="${band}" fill="${C.night}"/>`);
  out.push(
    bold(M, y + 52, "MONTHLY CLIMATE REPORT", 20, C.teal, { spacing: 2.4 }),
  );
  out.push(bold(M, y + 112, title, 56, "#e7eef4"));
  out.push(t(M, y + 154, subtitle, { size: 22, fill: "#b9c5d0" }));
  out.push(
    `<image href="data:image/svg+xml;base64,${logo}" x="${W - M - 120}" y="${y + 30}" width="44" height="44"/>`,
  );
  out.push(
    bold(W - M, y + 62, "WBY", 26, "#e7eef4", {
      anchor: "end",
      spacing: 2.2,
    }),
  );
  y += band;
}

function heroTiles(tiles) {
  y += 56;
  const w = (PW - 2 * 24) / 3;
  tiles.forEach((h, i) => {
    const x = M + i * (w + 24);
    out.push(card(x, y, w, 176));
    out.push(bold(x + 28, y + 86, h.big, 62, h.color ?? C.ink));
    out.push(bold(x + 28, y + 124, h.label, 19));
    out.push(t(x + 28, y + 152, h.sub, { size: 17, fill: C.pencil }));
  });
  y += 176;
}

function section(title, sub) {
  y += 84;
  out.push(bold(M, y, title, 32));
  if (sub) {
    y += 34;
    out.push(t(M, y, sub, { size: 19, fill: C.pencil }));
  }
  y += 28;
}

function yAxis(sy, ticks, label, zero) {
  for (const v of ticks) {
    const base = zero && v === 0;
    out.push(hline(M, M + PW, sy(v), base ? C.pencil : C.grid, base ? 1.5 : 1));
    out.push(
      t(M - 12, sy(v) + 6, label(v), {
        size: 16,
        fill: C.faint,
        anchor: "end",
      }),
    );
  }
}

function dayAxis(cx, bottom, monthAbbr) {
  for (const day of [1, 8, 15, 22, 29]) {
    out.push(
      t(cx(day - 1), bottom + 30, `${monthAbbr} ${day}`, {
        size: 16,
        fill: C.faint,
        anchor: "middle",
      }),
    );
  }
}

function yearsChart(data, { normal, lo, hi, ticks, notes: list }) {
  const years = data.years;
  const H = 300;
  const top = y + 20;
  const sy = (v) => top + ((hi - v) / (hi - lo)) * H;
  const step = PW / years.length;
  const bw = step - 3;
  yAxis(
    sy,
    ticks,
    (v) => `${v > 0 ? "+" : v < 0 ? "−" : ""}${Math.abs(v)}°`,
    true,
  );
  const last = years.at(-1).year;
  years.forEach((yr, i) => {
    const a = yr.temp_avg - normal;
    const x = M + i * step + 1.5;
    const fill = a >= 0 ? (yr.year === last ? C.warm : C.warmSoft) : C.coolSoft;
    const h = Math.max(Math.abs(sy(a) - sy(0)), 1.5);
    out.push(
      `<rect x="${x}" y="${sy(Math.max(a, 0))}" width="${bw}" height="${h}" rx="2" fill="${fill}"/>`,
    );
    if (yr.year % 10 === 0 || i === 0) {
      out.push(
        t(x + bw / 2, top + H + 30, yr.year, {
          size: 16,
          fill: C.faint,
          anchor: "middle",
        }),
      );
    }
  });
  const first = years[0].year;
  notes(list, (year) => M + (year - first) * step + 1.5 + bw / 2, sy);
  y = top + H + 30;
}

function dailyChart(data, { lo, hi, ticks, monthAbbr, notes: list, caption }) {
  const days = data.days;
  const H = 360;
  const top = y + 20;
  const sy = (v) => top + ((hi - v) / (hi - lo)) * H;
  const step = PW / days.length;
  const cx = (i) => M + (i + 0.5) * step;
  yAxis(sy, ticks, (v) => `${v}°`);
  for (let i = 0; i < days.length - 1; i++) {
    const a = days[i];
    const b = days[i + 1];
    const da = a.temp_avg - a.n96.temp_avg;
    const db = b.temp_avg - b.n96.temp_avg;
    const pts = [[i, a.temp_avg, a.n96.temp_avg]];
    if (da * db < 0) {
      const f = da / (da - db);
      const nv = a.n96.temp_avg + f * (b.n96.temp_avg - a.n96.temp_avg);
      pts.push([i + f, nv, nv]);
    }
    pts.push([i + 1, b.temp_avg, b.n96.temp_avg]);
    for (let k = 0; k < pts.length - 1; k++) {
      const [i0, v0, n0] = pts[k];
      const [i1, v1, n1] = pts[k + 1];
      const above = v0 + v1 - n0 - n1 >= 0;
      out.push(
        `<path d="M${cx(i0)},${sy(v0)} L${cx(i1)},${sy(v1)} L${cx(i1)},${sy(n1)} L${cx(i0)},${sy(n0)} Z" fill="${above ? C.warmWash : C.coolWash}"/>`,
      );
    }
  }
  days.forEach((d, i) => {
    out.push(
      `<line x1="${cx(i)}" x2="${cx(i)}" y1="${sy(d.temp_high)}" y2="${sy(d.temp_low)}" stroke="#c9bfae" stroke-width="5" stroke-linecap="round"/>`,
    );
  });
  const line = (key) =>
    days.map((d, i) => `${i ? "L" : "M"}${cx(i)},${sy(key(d))}`).join(" ");
  out.push(
    `<path d="${line((d) => d.n96.temp_avg)}" fill="none" stroke="${C.pencil}" stroke-width="2" stroke-dasharray="6 5"/>`,
  );
  out.push(
    `<path d="${line((d) => d.temp_avg)}" fill="none" stroke="${C.warm}" stroke-width="3" stroke-linejoin="round"/>`,
  );
  days.forEach((d, i) => {
    out.push(
      `<circle cx="${cx(i)}" cy="${sy(d.temp_avg)}" r="4" fill="${C.warm}" stroke="${C.paper}" stroke-width="2"/>`,
    );
  });
  dayAxis(cx, top + H, monthAbbr);
  notes(list, (day) => cx(day - 1), sy);
  const lx = M + PW - 360;
  const ly = top - 4;
  out.push(hline(lx, lx + 28, ly, C.warm, 3));
  out.push(t(lx + 36, ly + 6, "2026 daily mean", { size: 16, fill: C.pencil }));
  out.push(
    hline(lx + 190, lx + 218, ly, C.pencil, 2, 'stroke-dasharray="6 5"'),
  );
  out.push(t(lx + 226, ly + 6, "Normal", { size: 16, fill: C.pencil }));
  y = top + H + 30 + 44;
  out.push(t(M, y, caption, { size: 19 }));
}

function tiles(list) {
  const w = (PW - 2 * 24) / 3;
  const h = 210;
  list.forEach((tile, i) => {
    const x = M + (i % 3) * (w + 24);
    const ty = y + Math.floor(i / 3) * (h + 24);
    out.push(card(x, ty, w, h));
    out.push(bold(x + 26, ty + 70, tile.big, 48, tile.color ?? C.ink));
    if (tile.unit) out.push(bold(x + 26, ty + 104, tile.unit, 18));
    tile.lines.forEach((l, k) => {
      out.push(
        t(x + 26, ty + (tile.unit ? 138 : 110) + k * 24, l, {
          size: 17,
          fill: C.pencil,
        }),
      );
    });
  });
  y += Math.ceil(list.length / 3) * (h + 24) - 24;
}

function rainChart(data, { hi, ticks, monthAbbr, notes: list }) {
  const days = data.days;
  const H = 280;
  const top = y + 20;
  const sy = (v) => top + ((hi - v) / hi) * H;
  const step = PW / days.length;
  const cx = (i) => M + (i + 0.5) * step;
  yAxis(sy, ticks, (v) => `${v}`, true);
  days.forEach((d, i) => {
    if (d.precip_mm <= 0) return;
    const h = Math.max(sy(0) - sy(d.precip_mm), 2);
    out.push(
      `<rect x="${cx(i) - step / 2 + 4}" y="${sy(0) - h}" width="${step - 8}" height="${h}" rx="2" fill="${C.coolSoft}"/>`,
    );
  });
  const line = (key) =>
    days.map((d, i) => `${i ? "L" : "M"}${cx(i)},${sy(key(d))}`).join(" ");
  out.push(
    `<path d="${line((d) => d.n91.precip_cum)}" fill="none" stroke="${C.pencil}" stroke-width="2" stroke-dasharray="6 5"/>`,
  );
  out.push(
    `<path d="${line((d) => d.precip_cum)}" fill="none" stroke="${C.rain}" stroke-width="3" stroke-linejoin="round"/>`,
  );
  notes(list, (day) => cx(day - 1), sy);
  dayAxis(cx, top + H, monthAbbr);
  y = top + H + 30;
}

function sunChart(data, { hi, ticks, monthAbbr, band, notes: list }) {
  const days = data.sunshine.primary.days;
  const H = 220;
  const top = y + 20;
  const sy = (v) => top + ((hi - v) / hi) * H;
  const step = PW / days.length;
  const cx = (i) => M + (i + 0.5) * step;
  if (band) {
    const [from, to] = band;
    out.push(
      `<rect x="${cx(from - 1) - step / 2}" y="${top - 6}" width="${step * (to - from + 1)}" height="${H + 6}" rx="8" fill="${C.sunWash}"/>`,
    );
  }
  yAxis(sy, ticks, (v) => `${v} h`, true);
  days.forEach((d, i) => {
    const h = Math.max(sy(0) - sy(d.sun_h), d.sun_h > 0 ? 2 : 0);
    if (h > 0)
      out.push(
        `<rect x="${cx(i) - step / 2 + 4}" y="${sy(0) - h}" width="${step - 8}" height="${h}" rx="2" fill="${C.sun}"/>`,
      );
    out.push(
      hline(
        cx(i) - step / 2 + 6,
        cx(i) + step / 2 - 6,
        sy(d.normal_h),
        C.ink,
        2,
      ),
    );
  });
  notes(list, (day) => cx(day - 1), sy);
  dayAxis(cx, top + H, monthAbbr);
  y = top + H + 30;
}

const STATION_NOTES = {
  "Helsinki-Vantaa": "Airport, 17 km north",
  Kumpula: "Inland, 3 km from the center",
  Kaisaniemi: "City center",
  Harmaja: "Lighthouse island",
};

function stations(data, rows) {
  const order = ["Helsinki-Vantaa", "Kumpula", "Kaisaniemi", "Harmaja"];
  const list = order.map((n) => data.stations.find((s) => s.name === n));
  const w = (PW - 3 * 20) / 4;
  const h = 250;
  list.forEach((s, i) => {
    const x = M + i * (w + 20);
    out.push(card(x, y, w, h));
    out.push(bold(x + 22, y + 40, s.name, 20));
    out.push(
      t(x + 22, y + 66, STATION_NOTES[s.name], { size: 15, fill: C.pencil }),
    );
    out.push(bold(x + 22, y + 128, `${fmt(s.temp_avg)} °C`, 42, C.warm));
    out.push(t(x + 22, y + 152, "mean", { size: 15, fill: C.pencil }));
    out.push(hline(x + 22, x + w - 22, y + 172, C.grid, 1.5));
    rows.forEach((row, k) => {
      const ry = y + 202 + k * 28;
      out.push(t(x + 22, ry, row.label, { size: 16, fill: C.pencil }));
      out.push(
        bold(x + w - 22, ry, row.value(s), 18, row.color?.(s) ?? C.ink, {
          anchor: "end",
        }),
      );
    });
  });
  y += h;
}

function windFacts(data, facts) {
  const top = y;
  const r = 130;
  const ccx = M + r + 40;
  const ccy = top + r + 30;
  const share = data.wind.sector_share;
  const dirs = ["N", "NE", "E", "SE", "S", "SW", "W", "NW"];
  const max = 25;
  for (const v of [10, 20]) {
    out.push(
      `<circle cx="${ccx}" cy="${ccy}" r="${(v / max) * r}" fill="none" stroke="${C.grid}" stroke-width="1.2"/>`,
    );
  }
  out.push(
    t(ccx + 4, ccy - (20 / max) * r - 4, "20%", { size: 13, fill: C.faint }),
  );
  dirs.forEach((d, i) => {
    const ang = ((i * 45 - 90) * Math.PI) / 180;
    const half = (18 * Math.PI) / 180;
    const len = (share[d] / max) * r;
    const southerly = ["SE", "S", "SW"].includes(d);
    const p = (a, rr) => `${ccx + Math.cos(a) * rr},${ccy + Math.sin(a) * rr}`;
    out.push(
      `<path d="M${ccx},${ccy} L${p(ang - half, len)} A${len},${len} 0 0 1 ${p(ang + half, len)} Z" fill="${southerly ? C.warm : C.warmSoft}" stroke="${C.paper}" stroke-width="2"/>`,
    );
    const lr = r + 22;
    out.push(
      bold(
        ccx + Math.cos(ang) * lr,
        ccy + Math.sin(ang) * lr + 6,
        d,
        16,
        C.pencil,
        {
          anchor: "middle",
        },
      ),
    );
  });
  const tx = ccx + r + 110;
  facts.forEach(([big, txt], k) => {
    const fy = top + 48 + k * 74;
    out.push(bold(tx, fy, big, 30, C.warm));
    out.push(t(tx, fy + 28, txt, { size: 18, fill: C.pencil }));
  });
  y = top + 2 * r + 60;
}

function compare(columns) {
  const w = (PW - 24) / 2;
  const h = 360;
  columns.forEach((col, i) => {
    const x = M + i * (w + 24);
    out.push(card(x, y, w, h));
    out.push(bold(x + 30, y + 48, col.title, 24, col.accent));
    out.push(t(x + 30, y + 76, col.sub, { size: 17, fill: C.pencil }));
    col.rows.forEach(([big, txt], k) => {
      const ry = y + 140 + k * 82;
      out.push(bold(x + 30, ry, big, 34, col.accent));
      out.push(t(x + 30, ry + 28, txt, { size: 17, fill: C.pencil }));
    });
  });
  y += h;
}

function footer(url) {
  y += 50;
  const h = 120;
  out.push(`<rect y="${y}" width="${W}" height="${h}" fill="${C.night}"/>`);
  out.push(
    t(
      M,
      y + 50,
      "Data: Finnish Meteorological Institute open data. Normals 1991–2020 unless noted.",
      { size: 18, fill: "#b9c5d0" },
    ),
  );
  out.push(t(M, y + 80, `Full report: ${url}`, { size: 18, fill: "#b9c5d0" }));
  out.push(
    `<image href="data:image/svg+xml;base64,${logo}" x="${W - M - 120}" y="${y + 38}" width="44" height="44"/>`,
  );
  out.push(
    bold(W - M, y + 70, "WBY", 26, "#e7eef4", { anchor: "end", spacing: 2.2 }),
  );
  y += h;
}

function september(data) {
  const obs = data.observed;
  const n91 = data.normals["1991-2020"];
  header(
    heroScene("Helsinki, September 2026", ["wet!", "mild"]),
    "September 2026 in Helsinki",
    "Wet and mild: Helsinki's wettest September since 2012",
  );
  heroTiles([
    {
      big: `${fmt(obs.precip_mm)} mm`,
      color: C.rain,
      label: "Rain at Kaisaniemi",
      sub: `161% of the normal ${fmt(n91.precip_mm)} mm`,
    },
    {
      big: "16",
      color: C.rain,
      label: "Days with 1 mm or more",
      sub: "Almost twice the normal 8.6",
    },
    {
      big: `${fmt(obs.temp_avg)} °C`,
      color: C.warm,
      label: `Mean, ${signed(1.4)} °C above normal`,
      sub: "7th-warmest September since 1961",
    },
  ]);

  section(
    "Rain arrived late",
    "Kaisaniemi, millimeters: daily bars and the running month total against the normal",
  );
  rainChart(data, {
    hi: 100,
    ticks: [0, 20, 40, 60, 80, 100],
    monthAbbr: "Sep",
    notes: [
      {
        at: 30,
        v: 89.8,
        dx: 6,
        dy: -14,
        anchor: "end",
        size: 28,
        color: C.rain,
        text: "2026: 89.8 mm",
      },
      {
        at: 30,
        v: 40,
        dx: 6,
        anchor: "end",
        size: 28,
        text: "normal 55.7 mm",
      },
      {
        at: 23.4,
        v: 76,
        anchor: "end",
        size: 28,
        color: C.rain,
        text: "Sep 24–26: 38.4 mm in three days",
      },
      {
        at: 23.4,
        v: 66,
        anchor: "end",
        size: 24,
        text: "16.1 mm on the 25th, a record for the date",
      },
    ],
  });

  section(
    "Every September since 1961",
    "Difference from the 1991–2020 normal (12.3 °C), Helsinki Kaisaniemi",
  );
  yearsChart(data, {
    normal: 12.3,
    lo: -5,
    hi: 5,
    ticks: [-4, -2, 0, 2, 4],
    notes: [
      {
        at: 2026,
        v: 4.3,
        dx: 6,
        anchor: "end",
        size: 32,
        color: C.warm,
        text: "2026: +1.4 °C, the coolest of the last four",
        line: [4.0, 1.7],
      },
      {
        at: 1993,
        v: -4.6,
        dx: 12,
        dy: 6,
        size: 26,
        text: "1993: −4.6 °C",
      },
    ],
  });

  section(
    "Day by day",
    "Daily mean temperature against the 1996–2025 normal; bars span each day's low to high",
  );
  dailyChart(data, {
    lo: 4,
    hi: 22,
    ticks: [5, 10, 15, 20],
    monthAbbr: "Sep",
    caption:
      "22 of 30 days were warmer than normal, nine of them by 2 °C or more.",
    notes: [
      { at: 2, v: 20.1, dx: 12, dy: -6, text: "20.1 °C on the 2nd" },
      {
        at: 7.6,
        v: 5.6,
        color: C.cool,
        text: "7th: −2.6 °C, the coolest vs. normal",
      },
      {
        at: 29,
        v: 19.4,
        dy: -8,
        anchor: "middle",
        color: C.warm,
        text: "+3.8 °C on the 29th",
        line: [16.4, 19.4],
      },
    ],
  });

  section("The month in numbers");
  tiles([
    {
      big: "0",
      unit: "nights below 5 °C",
      color: C.cool,
      lines: [
        "A normal September has 3–4.",
        "Only the eighth time since 1961;",
        "the last was 2023.",
      ],
    },
    {
      big: "20.1 °C",
      unit: "highest, on the 2nd",
      color: C.warm,
      lines: [
        "The only day to reach 20 °C,",
        "where a typical September",
        "has two.",
      ],
    },
    {
      big: "7",
      unit: "heating degree days",
      color: C.warm,
      lines: [
        "A normal September has 63.",
        "Only 1992 and 2023 needed less.",
        "",
      ],
    },
    {
      big: "11.4 °C",
      unit: "feels-like",
      color: C.warm,
      lines: [
        "+1.3 °C against the normal.",
        "Nights stood out most, with",
        "feels-like lows 1.5 °C above.",
      ],
    },
    {
      big: "68",
      unit: "rainy hours from 8 am to 9 pm",
      color: C.rain,
      lines: [
        "Almost twice the usual 36,",
        "the most since the hourly",
        "record began in 2013.",
      ],
    },
    {
      big: "154.8 h",
      unit: "sunshine",
      color: C.sunDeep,
      lines: ["Helsinki-Vantaa, 104% of normal,", "despite all the rain.", ""],
    },
  ]);

  section(
    "Sun in all-or-nothing days",
    "Hours of sunshine each day at Helsinki-Vantaa; ticks mark the usual amount",
  );
  sunChart(data, {
    hi: 14,
    ticks: [0, 4, 8, 12],
    monthAbbr: "Sep",
    notes: [
      {
        at: 14,
        v: 11.8,
        dy: -10,
        anchor: "middle",
        size: 26,
        color: C.sunDeep,
        text: "11.8 h on the 14th",
      },
    ],
  });
  y += 44;
  out.push(
    t(
      M,
      y,
      "Nine days had less than half an hour of sun (usually five), and ten had eight hours or more (usually eight).",
      { size: 19 },
    ),
  );

  section(
    "Warmest at sea, coolest at the airport",
    "September mean temperature, lowest reading, and feels-like at four stations",
  );
  stations(data, [
    { label: "Lowest", value: (s) => `${fmt(s.abs_low)} °C` },
    { label: "Feels-like", value: (s) => `${fmt(s.feels_avg)} °C` },
  ]);

  section(
    "Why so mild: the cloud kept the nights warm",
    "Days sorted by sunshine at Helsinki-Vantaa; temperature and rain at Kaisaniemi",
  );
  compare([
    {
      title: "9 gray days",
      sub: "Less than half an hour of sun",
      accent: C.rain,
      rows: [
        ["+1.8 °C", "daily mean against normal"],
        ["+3.3 °C", "nightly lows, under the cloud"],
        ["54.6 mm", "of the month's rain"],
      ],
    },
    {
      title: "10 sunny days",
      sub: "Eight hours of sun or more",
      accent: C.sunDeep,
      rows: [
        ["+0.2 °C", "daily mean against normal"],
        ["−0.8 °C", "nightly lows, under clear skies"],
        ["2.7 mm", "of the month's rain"],
      ],
    },
  ]);

  footer("wb.yusif.fi/posts/2026/09/helsinki");
}

function october(data) {
  const obs = data.observed;
  const n91 = data.normals["1991-2020"];
  header(
    goldenParkScene("Helsinki, October 2026", ["mild", "no frost"]),
    "October 2026 in Helsinki",
    "Mild, frost-free, and dry: Helsinki Kaisaniemi, against the 1991–2020 normal",
  );
  heroTiles([
    {
      big: `${fmt(obs.temp_avg)} °C`,
      label: "Mean temperature",
      sub: `Normal ${fmt(n91.temp_avg)} °C`,
    },
    {
      big: `${signed(obs.temp_avg - n91.temp_avg)} °C`,
      color: C.warm,
      label: "Above the 1991–2020 normal",
      sub: `Running 1996–2025 normal: ${signed(obs.temp_avg - data.normals["1996-2025"].temp_avg)} °C`,
    },
    {
      big: "2nd",
      label: "Mildest October since 1961",
      sub: "Only 1961 was milder, at 10.2 °C",
    },
  ]);

  section(
    "Every October since 1961",
    "Difference from the 1991–2020 normal (6.6 °C), Helsinki Kaisaniemi",
  );
  yearsChart(data, {
    normal: n91.temp_avg,
    lo: -3,
    hi: 4,
    ticks: [-2, 0, 2, 4],
    notes: [
      {
        at: 2026,
        v: obs.temp_avg - n91.temp_avg,
        dx: -14,
        dy: -6,
        anchor: "end",
        size: 34,
        color: C.warm,
        text: "2026: +3.2 °C",
      },
      {
        at: 1961,
        v: 10.19 - n91.temp_avg,
        dx: 14,
        dy: 14,
        size: 28,
        text: "1961: +3.6 °C, the only milder one",
      },
      {
        at: 2000,
        v: 9.5 - n91.temp_avg,
        dy: -14,
        anchor: "middle",
        text: "2000",
      },
    ],
  });

  section(
    "Day by day",
    "Daily mean temperature against the 1996–2025 normal; bars span each day's low to high",
  );
  dailyChart(data, {
    lo: 0,
    hi: 18,
    ticks: [0, 5, 10, 15],
    monthAbbr: "Oct",
    caption:
      "25 of 31 days were milder than normal, 21 of them by 2 °C or more.",
    notes: [
      { at: 1, v: 16.2, dx: 10, dy: -8, text: "16.2 °C high on the 1st" },
      {
        at: 17.6,
        v: 1.6,
        anchor: "end",
        color: C.cool,
        text: "clear, chilly nights, Oct 18–20",
      },
      {
        at: 26,
        v: 15.6,
        dy: -8,
        anchor: "middle",
        color: C.warm,
        text: "+6.2 °C on the 26th",
        line: [12.2, 15.6],
      },
    ],
  });

  section("The month in numbers");
  tiles([
    {
      big: "0",
      unit: "frost days",
      color: C.cool,
      lines: [
        "Lowest reading in the center: 1.1 °C.",
        "Only 2008 and 2011 also stayed",
        "frost-free in 1991–2020.",
      ],
    },
    {
      big: "Oct 29",
      lines: [
        "Thermal autumn began, late.",
        "Median start: Oct 19 (1991–2020).",
      ],
    },
    {
      big: "117",
      unit: "heating degree days",
      color: C.warm,
      lines: ["A normal October has 286.", "Only 1961 needed less."],
    },
    {
      big: "6.8 °C",
      unit: "feels-like",
      color: C.warm,
      lines: [
        "+3.2 °C against the normal.",
        "Mildest-feeling October",
        "since at least 1996.",
      ],
    },
    {
      big: "44.9 mm",
      unit: "rain",
      color: C.rain,
      lines: [
        "61% of the normal 73.3 mm,",
        "though spread over 18 wet days",
        "(normal 16.3).",
      ],
    },
    {
      big: "78.1 h",
      unit: "sunshine",
      color: C.sunDeep,
      lines: [
        "Helsinki-Vantaa; average 84.2 h.",
        "Weekends got 1.5 h a day,",
        "weekdays 2.9 h.",
      ],
    },
  ]);

  section(
    "Rain came often, but lightly",
    "Kaisaniemi, millimeters: daily bars and the running month total against the normal",
  );
  rainChart(data, {
    hi: 80,
    ticks: [0, 20, 40, 60, 80],
    monthAbbr: "Oct",
    notes: [
      {
        at: 31,
        v: data.days.at(-1).n91.precip_cum,
        dx: -6,
        dy: -12,
        anchor: "end",
        size: 28,
        text: `normal ${fmt(n91.precip_mm)} mm`,
      },
      {
        at: 31,
        v: obs.precip_mm,
        dx: -6,
        dy: 46,
        anchor: "end",
        size: 28,
        color: C.rain,
        text: `2026: ${fmt(obs.precip_mm)} mm`,
      },
      {
        at: 9,
        v: 33,
        anchor: "middle",
        size: 24,
        text: "wettest day: 11.6 mm",
        line: [12.5, 31],
      },
    ],
  });

  section(
    "Almost half the sun in one clear spell",
    "Hours of sunshine each day at Helsinki-Vantaa; ticks mark the usual amount",
  );
  sunChart(data, {
    hi: 12,
    ticks: [0, 4, 8, 12],
    monthAbbr: "Oct",
    band: [18, 22],
    notes: [
      {
        at: 20.5,
        v: 10.5,
        anchor: "middle",
        size: 30,
        color: C.sunDeep,
        text: "Oct 18–22: 38.1 h",
      },
      {
        at: 28.5,
        v: 10.5,
        anchor: "middle",
        size: 28,
        text: "Oct 24–31: 3.9 h",
      },
    ],
  });

  section(
    "From the airport to the sea",
    "October mean temperature, lowest reading, and frosty nights at four stations",
  );
  stations(data, [
    {
      label: "Lowest",
      value: (s) => `${fmt(s.abs_low)} °C`,
      color: (s) => (s.abs_low < 0 ? C.cool : C.ink),
    },
    {
      label: "Frosty nights",
      value: (s) => `${s.frost_days}`,
      color: (s) => (s.frost_days > 0 ? C.cool : C.ink),
    },
  ]);

  section("Why so mild: southerly winds and cloud");
  windFacts(data, [
    ["54%", "of the time the wind blew from between southeast and southwest"],
    ["20 of 31", "days were overcast, so nights couldn't cool"],
    ["≈ +3 °C", "highs and lows alike, above the 1991–2020 normal"],
    ["18.5 m/s", "strongest gust, 2:30 pm on October 6"],
  ]);

  footer("wb.yusif.fi/posts/2026/10/helsinki");
}

const posts = {
  "2026/09/helsinki": september,
  "2026/10/helsinki": october,
};

const fontFace = (family, file, type) =>
  `@font-face{font-family:${family};font-weight:700;src:url(data:font/${type};base64,${readFileSync(path(file)).toString("base64")}) format("${type === "ttf" ? "truetype" : type}")}`;
const webFonts =
  fontFace("Inter", "scripts/fonts/Inter-Bold.ttf", "ttf") +
  fontFace("Caveat", "public/fonts/caveat-latin.woff2", "woff2");

const only = process.argv[2];
for (const [id, build] of Object.entries(posts)) {
  if (only && id !== only) continue;
  out = [];
  y = 0;
  const data = JSON.parse(
    readFileSync(path(`src/content/posts/${id}.json`), "utf8"),
  );
  build(data);
  const H = y;
  const [year, month, slug] = id.split("/");
  const title = `${new Date(`${year}-${month}-01T12:00:00Z`).toLocaleString("en-US", { month: "long", timeZone: "UTC" })} ${year} in Helsinki: monthly climate report infographic`;
  const document = (
    fonts,
  ) => `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
<title>${xml(title)}</title>
<style>${fonts}${style}</style>
<defs>
<filter id="shadow" x="-10%" y="-10%" width="120%" height="130%"><feDropShadow dx="0" dy="1.5" stdDeviation="0.6" flood-color="#000000" flood-opacity="0.45"/></filter>
<linearGradient id="rain-fade" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fff" stop-opacity="0"/><stop offset="0.14" stop-color="#fff"/><stop offset="0.88" stop-color="#fff"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></linearGradient>
<pattern id="grain" patternUnits="userSpaceOnUse" width="320" height="320"><image href="data:image/png;base64,${grain}" width="320" height="320"/></pattern>
</defs>
<rect width="${W}" height="${H}" fill="${C.paper}"/>
${out.join("\n")}
</svg>`;

  const png = new Resvg(document(""), {
    fitTo: { mode: "zoom", value: 2 },
    font: {
      fontFiles: [
        path("scripts/fonts/Caveat-Bold.ttf"),
        path("scripts/fonts/Inter-Bold.ttf"),
        "/System/Library/Fonts/HelveticaNeue.ttc",
      ],
      loadSystemFonts: false,
      defaultFontFamily: "Inter",
    },
  })
    .render()
    .asPng();
  const base = `src/content/posts/${year}/${month}/${slug}-${year}-${month}-infographic`;
  writeFileSync(
    path(`${base}.png`),
    await sharp(png)
      .png({ palette: true, quality: 95, effort: 10, dither: 1 })
      .toBuffer(),
  );
  writeFileSync(path(`${base}.svg`), document(webFonts));
  console.log(`${base}.{svg,png} (${W}×${H})`);
}
