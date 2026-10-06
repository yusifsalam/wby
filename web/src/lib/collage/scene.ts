import { type Rnd, rng } from "./sketch";

export type Enter = {
  delay: number;
  x?: number;
  y?: number;
  rotate?: number;
  kind?: "place" | "drop" | "grow" | "fade" | "swing";
};

export type Layer = {
  x: number;
  y: number;
  w: number;
  h: number;
  svg: string;
  className?: string;
  enter?: Enter;
  loop?: string;
  vars?: Record<string, string | number>;
  depth?: number;
};

export type Scene = { w: number; h: number; layers: Layer[] };

// A path with fill and/or stroke tone classes.
export function path(
  d: string,
  fill?: string,
  stroke?: string,
  extra = "",
): string {
  const cls = [fill && `f-${fill}`, stroke && `s-${stroke}`]
    .filter(Boolean)
    .join(" ");
  return `<path class="${cls}" d="${d}"${fill ? "" : ' fill="none"'}${extra ? ` ${extra}` : ""}/>`;
}

export function pencil(
  d: string,
  tone = "ink",
  width: number | string = 1.4,
  extra = "",
  className = "",
): string {
  return `<path class="s-${tone}${className ? ` ${className}` : ""}" d="${d}" fill="none" stroke-width="${width}" stroke-linecap="round" stroke-linejoin="round"${extra ? ` ${extra}` : ""}/>`;
}

// Three takes of the same drawing that the stylesheet cycles through at
// stop-motion speed ("line boil").
export function boil(
  draw: (rnd: Rnd) => string,
  seed: number,
  className = "",
): string {
  const frames = [0, 1, 2]
    .map((i) => `<g class="cl-f cl-f${i}">${draw(rng(seed * 7 + i * 101))}</g>`)
    .join("");
  return `<g class="cl-boil${className ? ` ${className}` : ""}">${frames}</g>`;
}

export function group(content: string, attrs = ""): string {
  return `<g${attrs ? ` ${attrs}` : ""}>${content}</g>`;
}

export function text(
  x: number,
  y: number,
  content: string,
  size: number,
  tone = "ink",
  extra = "",
): string {
  return `<text class="cl-hand f-${tone}" x="${x}" y="${y}" font-size="${size}"${extra ? ` ${extra}` : ""}>${content}</text>`;
}

export function layerStyle(scene: Scene, layer: Layer): string {
  const pct = (v: number, of: number) =>
    `${Math.round((v / of) * 100000) / 1000}%`;
  const parts = [
    `left:${pct(layer.x, scene.w)}`,
    `top:${pct(layer.y, scene.h)}`,
    `width:${pct(layer.w, scene.w)}`,
    `height:${pct(layer.h, scene.h)}`,
  ];
  if (layer.enter) {
    parts.push(`--d:${layer.enter.delay}s`);
    if (layer.enter.x) parts.push(`--fx:${layer.enter.x}%`);
    if (layer.enter.y) parts.push(`--fy:${layer.enter.y}%`);
    if (layer.enter.rotate) parts.push(`--fr:${layer.enter.rotate}deg`);
  }
  if (layer.depth) parts.push(`--depth:${layer.depth}`);
  for (const [k, v] of Object.entries(layer.vars ?? {}))
    parts.push(`--${k}:${v}`);
  return parts.join(";");
}

export function enterClass(layer: Layer): string {
  return layer.enter ? `cl-in cl-in--${layer.enter.kind ?? "place"}` : "";
}
