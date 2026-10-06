// Collage colours as [light, dark]. Pieces reference them through f-<name>
// (fill) and s-<name> (stroke) classes so the dark theme can re-tint the
// paper without touching the drawings.
export const palette = {
  ink: ["#25303c", "#1e2731"],
  pencil: ["#5d6b79", "#55606c"],
  note: ["#25303c", "#ece6d8"],
  noteSoft: ["#5d6b79", "#c9c3b5"],
  paper: ["#fbf6ec", "#d9d2c3"],
  fiber: ["#fffdf8", "#e8e2d4"],
  kraft: ["#cfae80", "#a68a63"],
  kraftDark: ["#b08d5e", "#8a7050"],
  notebook: ["#fdfcf7", "#d7d5cc"],
  rule: ["#9cc0e0", "#7d9ab3"],
  margin: ["#e58b8b", "#b87171"],
  sky: ["#d6e4ee", "#5d7184"],
  skyDeep: ["#b8cddd", "#4b5f72"],
  cloud: ["#f6f8fa", "#c3ccd5"],
  cloudShade: ["#dfe5eb", "#a7b2bd"],
  rainCloud: ["#8a9cae", "#66788a"],
  rainCloudShade: ["#71849a", "#56687a"],
  rain: ["#4c7fbf", "#79a6dc"],
  water: ["#7fb0de", "#5f8fbf"],
  waterDeep: ["#4f86c1", "#4877a8"],
  glass: ["#e9f2f7", "#93a6b6"],
  sun: ["#f6b93b", "#c99530"],
  sunDeep: ["#ec9a1e", "#b67c1c"],
  orange: ["#ea580c", "#d4500d"],
  orangeSoft: ["#f7a26b", "#c47a4e"],
  blue: ["#2563eb", "#3b6fd6"],
  blueSoft: ["#93b4f3", "#6f8fcc"],
  slate: ["#64748b", "#6c7a8d"],
  copper: ["#5f9f88", "#4f8571"],
  copperDark: ["#477e6a", "#3d6857"],
  wall: ["#f3eee3", "#c9c2b4"],
  wallShade: ["#ddd5c4", "#ada595"],
  granite: ["#a99d90", "#857a6f"],
  graniteDark: ["#887c70", "#6c6258"],
  block: ["#a9bcb9", "#6f8481"],
  blockDark: ["#8ca39f", "#5d716e"],
  blockWarm: ["#d6b9a0", "#9c8471"],
  window: ["#f2e3b8", "#c5ad6c"],
  grass: ["#86a55c", "#617a43"],
  grassDark: ["#678643", "#4d6534"],
  path: ["#dcc8a6", "#a39274"],
  birch: ["#f3c13a", "#c99a2c"],
  maple: ["#e4762d", "#b85d25"],
  rowan: ["#c94a2c", "#9e3b25"],
  leafGreen: ["#9bb35a", "#71843f"],
  bark: ["#efe9de", "#bdb6a9"],
  trunk: ["#6d5a47", "#54463a"],
  coat: ["#f2c230", "#c79f2a"],
  umbrella: ["#e2553b", "#b8452f"],
  skin: ["#f1c9a5", "#c4a283"],
  scarf: ["#d8452f", "#ad3a28"],
  radiator: ["#eef0f2", "#b3b9bf"],
  stamp: ["#c8372d", "#b83a30"],
  tram: ["#4f9a52", "#3e7a41"],
  tramLight: ["#f4ead2", "#c7bea8"],
  white: ["#ffffff", "#e3e0d8"],
  tapeOrange: ["rgba(240, 120, 50, 0.62)", "rgba(214, 110, 52, 0.55)"],
  tapeBlue: ["rgba(90, 140, 230, 0.5)", "rgba(100, 140, 220, 0.45)"],
  tapeAmber: ["rgba(246, 185, 59, 0.66)", "rgba(220, 170, 60, 0.55)"],
  tapeMint: ["rgba(120, 190, 160, 0.6)", "rgba(110, 170, 145, 0.5)"],
  tapeCream: ["rgba(250, 244, 226, 0.78)", "rgba(220, 212, 190, 0.6)"],
} as const;

export type Tone = keyof typeof palette;

function kebab(name: string): string {
  return name.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`);
}

// Custom properties for both themes plus one fill and one stroke class per
// colour, scoped to collage elements.
export function paletteCss(): string {
  const entries = Object.entries(palette) as [
    Tone,
    readonly [string, string],
  ][];
  const light = entries.map(([k, [v]]) => `--cl-${kebab(k)}:${v};`).join("");
  const dark = entries.map(([k, [, v]]) => `--cl-${kebab(k)}:${v};`).join("");
  const classes = entries
    .map(
      ([k]) =>
        `.cl .f-${kebab(k)}{fill:var(--cl-${kebab(k)})}.cl .s-${kebab(k)}{stroke:var(--cl-${kebab(k)})}`,
    )
    .join("");
  return `:root{${light}}:root.dark{${dark}}${classes}`;
}

// The same classes resolved to literal colours, for rendering previews with
// tools that don't support custom properties.
export function paletteLiteralCss(theme: 0 | 1 = 0): string {
  return (Object.entries(palette) as [Tone, readonly [string, string]][])
    .map(
      ([k, v]) =>
        `.f-${kebab(k)}{fill:${v[theme]}}.s-${kebab(k)}{stroke:${v[theme]}}`,
    )
    .join("");
}
