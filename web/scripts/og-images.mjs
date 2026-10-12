// Renders each report's social preview image: the hero collage above a strip
// with the kicker, title and WBY mark. Run with `npm run og` and commit the
// images next to the posts.
import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { Resvg } from "@resvg/resvg-js";
import sharp from "sharp";
import { runnerImport } from "vite";

const W = 1200;
const H = 630;
const STRIP = 110;

const path = (p) => fileURLToPath(new URL(`../${p}`, import.meta.url));
const load = async (p) =>
  (await runnerImport(path(p), { configFile: false, logLevel: "error" }))
    .module;

const { paletteLiteralCss } = await load("src/lib/collage/palette.ts");
const { heroScene } = await load("src/lib/collage/scenes/hero.ts");

const posts = {
  "2026/09/helsinki": heroScene,
};

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

function xml(text) {
  return text
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function frontmatter(id) {
  const mdx = readFileSync(path(`src/content/posts/${id}.mdx`), "utf8");
  const block = mdx.match(/^---\n([\s\S]*?)\n---/)[1];
  const field = (key) =>
    block.match(new RegExp(`^${key}: (.*)$`, "m"))[1].replace(/^"(.*)"$/, "$1");
  return { kicker: field("kicker"), title: field("title") };
}

function collage(scene) {
  const layers = scene.layers.map((layer, i) => {
    const classes = (layer.className ?? "").split(" ");
    if (classes.includes("cl-leaf") || classes.includes("cl-walker")) return "";
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

function card(scene, { kicker, title }) {
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
<style>${style}</style>
<defs>
<filter id="shadow" x="-10%" y="-10%" width="120%" height="130%"><feDropShadow dx="0" dy="1.5" stdDeviation="0.6" flood-color="#000000" flood-opacity="0.45"/></filter>
<linearGradient id="rain-fade" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fff" stop-opacity="0"/><stop offset="0.14" stop-color="#fff"/><stop offset="0.88" stop-color="#fff"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></linearGradient>
<pattern id="grain" patternUnits="userSpaceOnUse" width="320" height="320"><image href="data:image/png;base64,${grain}" width="320" height="320"/></pattern>
</defs>
${collage(scene)}
<rect y="${H - STRIP}" width="${W}" height="${STRIP}" fill="#17212b"/>
<text x="48" y="${H - 72}" font-family="Inter" font-weight="700" font-size="22" fill="#2dd4bf">${xml(kicker)}</text>
<text x="48" y="${H - 27}" font-family="Inter" font-weight="700" font-size="40" fill="#e7eef4">${xml(title)}</text>
<image href="data:image/svg+xml;base64,${logo}" x="1028" y="${H - 79}" width="48" height="48"/>
<text x="1086" y="${H - 45}" font-family="Inter" font-weight="700" font-size="28" letter-spacing="2.2" fill="#e7eef4">WBY</text>
</svg>`;
}

for (const [id, scene] of Object.entries(posts)) {
  const svg = card(scene(), frontmatter(id));
  const png = new Resvg(svg, {
    font: {
      fontFiles: [
        path("scripts/fonts/Caveat-Bold.ttf"),
        path("scripts/fonts/Inter-Bold.ttf"),
      ],
      loadSystemFonts: false,
      defaultFontFamily: "Inter",
    },
  })
    .render()
    .asPng();
  const jpeg = await sharp(png).jpeg({ quality: 90, mozjpeg: true }).toBuffer();
  const out = `src/content/posts/${id}-og.jpg`;
  writeFileSync(path(out), jpeg);
  console.log(`${out} (${Math.round(jpeg.length / 1024)} KB)`);
}
