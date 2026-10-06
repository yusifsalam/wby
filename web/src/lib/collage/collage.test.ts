import { describe, expect, it } from "vitest";
import data from "../../content/posts/2026/09/helsinki.json";
import type { MonthReport } from "../reportChart";
import { paletteCss } from "./palette";
import { cutRect, tape, torn } from "./paper";
import { boil, layerStyle } from "./scene";
import {
  calendarScene,
  rainJarScene,
  recorderScene,
  rulersScene,
} from "./scenes/doodles";
import { heroScene } from "./scenes/hero";
import { transectScene } from "./scenes/transect";
import { cloudShape, curve, line, rng } from "./sketch";

const report = data as unknown as MonthReport;

describe("sketch", () => {
  it("draws the same lines for the same seed", () => {
    expect(line([0, 0], [100, 0], rng(1))).toBe(line([0, 0], [100, 0], rng(1)));
    expect(line([0, 0], [100, 0], rng(1))).not.toBe(
      line([0, 0], [100, 0], rng(2)),
    );
  });

  it("draws a double pencil pass by default", () => {
    expect(line([0, 0], [100, 0], rng(3)).match(/M/g)).toHaveLength(2);
    expect(
      line([0, 0], [100, 0], rng(3), { double: false }).match(/M/g),
    ).toHaveLength(1);
  });

  it("writes compact one-decimal coordinates", () => {
    expect(
      curve([
        [0, 0],
        [10.04, 5],
        [20, 0],
      ]),
    ).toBe("M0 0C1.7 0.8 6.7 5 10 5C13.4 5 18.3 0.8 20 0");
    expect(cutRect(0, 0, 10, 10, rng(4))).not.toMatch(/\d\.\d\d/);
  });

  it("keeps clouds inside their box, sitting on a base", () => {
    const pts = cloudShape(10, 20, 200, 80, rng(5), 5);
    for (const [x, y] of pts) {
      expect(x).toBeGreaterThanOrEqual(9);
      expect(x).toBeLessThanOrEqual(211);
      expect(y).toBeGreaterThanOrEqual(19);
      expect(y).toBeLessThanOrEqual(102);
    }
  });

  it("closes torn and taped paper shapes", () => {
    expect(
      torn(
        [
          [0, 0],
          [50, 0],
          [50, 50],
          [0, 50],
        ],
        rng(6),
      ),
    ).toMatch(/Z$/);
    expect(tape(10, 10, 40, 12, 20, rng(7))).toMatch(/Z$/);
  });
});

describe("scene", () => {
  it("cycles three takes of a boiling drawing", () => {
    const g = boil(() => "<path/>", 9);
    expect(g.match(/class="cl-f cl-f\d"/g)).toEqual([
      'class="cl-f cl-f0"',
      'class="cl-f cl-f1"',
      'class="cl-f cl-f2"',
    ]);
  });

  it("places layers in percentages of the stage with their entrance", () => {
    const style = layerStyle(
      { w: 200, h: 100, layers: [] },
      { x: 50, y: 25, w: 100, h: 50, svg: "", enter: { delay: 0.4, y: -30 } },
    );
    expect(style).toBe(
      "left:25%;top:25%;width:50%;height:50%;--d:0.4s;--fy:-30%",
    );
  });

  it("emits a fill and stroke class for every colour in both themes", () => {
    const css = paletteCss();
    expect(css).toContain(":root.dark{");
    expect(css).toContain(".cl .f-tape-amber{fill:var(--cl-tape-amber)}");
    expect(css).toContain(".cl .s-ink{stroke:var(--cl-ink)}");
  });
});

describe("scenes", () => {
  it("builds the hero from many pieces with the walker and falling leaves", () => {
    const scene = heroScene();
    expect(scene.layers.length).toBeGreaterThan(25);
    expect(scene.layers.filter((l) => l.className === "cl-leaf")).toHaveLength(
      7,
    );
    expect(scene.layers.some((l) => l.className === "cl-walker")).toBe(true);
  });

  it("tears off every calendar page but the last", () => {
    const svg = calendarScene(report.days).layers[1].svg;
    expect(svg.match(/class="cl-tear"/g)).toHaveLength(report.days.length - 1);
    expect(svg).toContain(">30</text>");
  });

  it("fills the rain jar in one keyframe per day", () => {
    const svg = rainJarScene(report.days, 55.7, 161).layers[0].svg;
    const frames = svg.match(/\d+(\.\d+)?%\{transform:translateY\(/g) ?? [];
    expect(frames).toHaveLength(report.days.length + 1);
    expect(svg).toContain("translateY(0px)}}");
  });

  it("unfolds both rulers and labels both normals", () => {
    const svg = rulersScene(12.3, 12.65).layers[0].svg;
    expect(svg.match(/class="cl-unfold"/g)).toHaveLength(4);
    expect(svg).toContain("12.3°");
    expect(svg).toContain("12.65°");
  });

  it("fills one thermometer per station, warmest by the sea", () => {
    const scene = transectScene(report.stations);
    const tops = scene.layers
      .filter((l) => l.svg.includes("cl-mercury"))
      .map((l) =>
        Number(l.svg.match(/class="f-orange" d="M[\d.]+ ([\d.]+)/)?.[1]),
      );
    expect(tops).toHaveLength(4);
    expect(Math.min(...tops)).toBe(tops[3]);
  });

  it("scorches one mark per sunny hour, heavier for sunnier hours", () => {
    const profile = Array.from({ length: 24 }, (_, h) =>
      h >= 7 && h <= 18 ? (h === 12 ? 0.5 : 0.2) : 0,
    );
    const svg = recorderScene(profile, 154.8).layers[2].svg;
    const marks = [
      ...svg.matchAll(/class="s-ink cl-draw"[^>]*stroke-width="([\d.]+)"/g),
    ].map((m) => Number(m[1]));
    expect(marks).toHaveLength(12);
    expect(Math.max(...marks)).toBe(marks[5]);
    expect(recorderScene(profile, 154.8).layers[3].svg).toContain("154.8 h");
  });
});
