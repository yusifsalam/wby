import { sunOcclusion } from "./noise";
import { SkyRenderer } from "./renderer";
import {
  coverage,
  hasPrecipitation,
  hasRain,
  motionRates,
  precipitationIntensity,
  prefersLightForeground,
  rainSlant,
  type SkyState,
  skyGradientVars,
  skyStyle,
  type Vec2,
  wind,
} from "./state";

const STORAGE_KEY = "sky-effects";
const SUN_FADE_DISTANCE = 140;

class PhaseClock {
  private anchorValue: Vec2 = [0, 0];

  constructor(
    private anchorTime: number,
    private rate: Vec2,
  ) {}

  value(time: number): Vec2 {
    const dt = time - this.anchorTime;
    return [
      this.anchorValue[0] + this.rate[0] * dt,
      this.anchorValue[1] + this.rate[1] * dt,
    ];
  }

  retarget(rate: Vec2, time: number): void {
    this.anchorValue = this.value(time);
    this.anchorTime = time;
    this.rate = rate;
  }
}

type Clocks = {
  start: number;
  drift: PhaseClock;
  rain: PhaseClock;
  snow: PhaseClock;
};

function makeClocks(state: SkyState, time: number): Clocks {
  const rates = motionRates(state);
  return {
    start: time,
    drift: new PhaseClock(time, rates.drift),
    rain: new PhaseClock(time, [rates.rain, 0]),
    snow: new PhaseClock(time, rates.snow),
  };
}

function strikeIntensity(t: number): number {
  if (t < 0 || t >= 1.2) return 0;
  const pulse = (onset: number, peak: number, decay: number) => {
    const x = t - onset;
    return x < 0 ? 0 : peak * Math.min(x / 0.015, 1) * Math.exp(-x / decay);
  };
  return Math.max(
    pulse(0, 1, 0.06),
    pulse(0.12, 0.55, 0.05),
    pulse(0.27, 0.85, 0.14),
  );
}

const seconds = () => performance.now() / 1000;

let container: HTMLElement | null = null;
let canvas: HTMLCanvasElement | null = null;
let renderer: SkyRenderer | null = null;
let unavailable = false;
let contextLost = false;
let size: Vec2 | null = null;
let live: SkyState;
let current: SkyState;
let clocks: Clocks;
let strike: { start: number; x: number } | null = null;
let strikeTimer: number | undefined;
let frameRequest = 0;
let lastFrame = 0;
let lastSky = Number.NEGATIVE_INFINITY;
let sunFade = 1;
let covered = false;
let motion = true;
let reduceMotion: MediaQueryList;

function enabled(): boolean {
  return document.documentElement.dataset.skyEffects !== "off";
}

function motionAllowed(): boolean {
  let setting: string | null = null;
  try {
    setting = localStorage.getItem("motion");
  } catch {}
  return setting ? setting === "on" : !reduceMotion.matches;
}

function scrollSunFade(): number {
  const fade = 1 - Math.min(Math.max(window.scrollY, 0) / SUN_FADE_DISTANCE, 1);
  return Math.round(fade * 12) / 12;
}

function frameRate(): number {
  if (hasPrecipitation(current.scene)) return 60;
  return sunFade > 0 ? 30 : 15;
}

function ensureRenderer(): boolean {
  if (!renderer && !unavailable && canvas) {
    try {
      renderer = SkyRenderer.create(canvas);
    } catch (error) {
      console.error(error);
    }
    if (renderer && size) renderer.resize(...size);
    unavailable = renderer == null;
  }
  return renderer != null && !contextLost;
}

function drawFrame(t: number, renderSky: boolean): void {
  if (!renderer || renderer.width === 0) return;
  const state = current;
  const style = skyStyle(state.scene, coverage(state));
  const time = t - clocks.start;
  const drift = clocks.drift.value(t);
  const occlusion =
    renderSky && style.sun * sunFade > 0.001
      ? sunOcclusion(
          renderer.width,
          renderer.height,
          time,
          drift,
          style.cumulus,
          style.deck,
        )
      : 0;
  const intensity = precipitationIntensity(state);
  const rainPeriod = 110 + 70 * Math.min(Math.max(intensity / 1.8, 0), 1);
  const fall = clocks.snow.value(t);
  renderer.render(
    {
      time,
      drift,
      clouds: [style.cumulus, style.deck, style.cirrus, style.absorption],
      light: [style.sun, sunFade, style.night, occlusion],
      lit: style.lit,
      shadow: style.shadow,
      flash: strike ? strikeIntensity(t - strike.start) : 0,
      flashX: strike?.x ?? 0.5,
      stars: style.stars,
      precipitation: hasRain(state.scene)
        ? "rain"
        : state.scene === "snow"
          ? "snow"
          : "none",
      rain: [clocks.rain.value(t)[0] % rainPeriod, intensity, rainSlant(state)],
      snow: [fall[0], fall[1], intensity, wind(state)[0]],
    },
    renderSky,
  );
  if (canvas && !("ready" in canvas.dataset)) canvas.dataset.ready = "";
}

function redraw(): void {
  if (!renderer || contextLost || !enabled() || document.hidden) return;
  lastSky = Number.NEGATIVE_INFINITY;
  drawFrame(motion ? seconds() : clocks.start, true);
}

function tick(timestamp: number): void {
  frameRequest = requestAnimationFrame(tick);
  if (timestamp - lastFrame < 1000 / frameRate() - 2) return;
  lastFrame = timestamp;
  const renderSky = timestamp - lastSky >= 1000 / 30 - 2;
  if (renderSky) lastSky = timestamp;
  drawFrame(timestamp / 1000, renderSky);
}

function startLoop(): void {
  if (frameRequest) return;
  lastFrame = 0;
  lastSky = Number.NEGATIVE_INFINITY;
  frameRequest = requestAnimationFrame(tick);
}

function stopLoop(): void {
  cancelAnimationFrame(frameRequest);
  frameRequest = 0;
}

function scheduleStrike(): void {
  strikeTimer = window.setTimeout(
    () => {
      strike = { start: seconds(), x: 0.15 + Math.random() * 0.7 };
      scheduleStrike();
    },
    (6 + Math.random() * 10) * 1000,
  );
}

function syncLightning(active: boolean): void {
  if (active && current.scene === "storm") {
    if (strikeTimer === undefined) scheduleStrike();
    return;
  }
  window.clearTimeout(strikeTimer);
  strikeTimer = undefined;
  strike = null;
}

function update(): void {
  const active = container != null && enabled() && ensureRenderer();
  const animating = active && motion && !covered && !document.hidden;
  if (animating) startLoop();
  else stopLoop();
  syncLightning(animating);
  if (active && !motion) redraw();
}

function restartClocks(): void {
  clocks = makeClocks(current, seconds());
}

function onResize(entries: ResizeObserverEntry[]): void {
  const box = entries[entries.length - 1].contentRect;
  if (box.width === 0 || box.height === 0) return;
  size = [box.width, box.height];
  renderer?.resize(box.width, box.height);
  redraw();
}

function onScroll(): void {
  const fade = scrollSunFade();
  if (fade === sunFade) return;
  sunFade = fade;
  lastSky = Number.NEGATIVE_INFINITY;
  if (!frameRequest) redraw();
}

function onDialogs(): void {
  const open = document.querySelector("dialog[open]") != null;
  if (open === covered) return;
  covered = open;
  update();
}

export function initSky(el: HTMLElement): void {
  if (container) return;
  container = el;
  canvas = el.querySelector("canvas");
  live = JSON.parse(el.dataset.sky ?? "") as SkyState;
  current = live;
  clocks = makeClocks(current, seconds());
  reduceMotion = matchMedia("(prefers-reduced-motion: reduce)");
  motion = motionAllowed();
  sunFade = scrollSunFade();

  reduceMotion.addEventListener("change", () => {
    motion = motionAllowed();
    update();
  });
  window.addEventListener("scroll", onScroll, { passive: true });
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) restartClocks();
    update();
  });
  window.addEventListener("pageshow", (event) => {
    if (!event.persisted) return;
    restartClocks();
    update();
  });
  new MutationObserver(onDialogs).observe(document.body, {
    subtree: true,
    attributes: true,
    attributeFilter: ["open"],
  });
  canvas?.addEventListener("webglcontextlost", (event) => {
    event.preventDefault();
    contextLost = true;
    update();
  });
  canvas?.addEventListener("webglcontextrestored", () => {
    contextLost = false;
    try {
      renderer?.setup();
    } catch (error) {
      console.error(error);
      renderer = null;
      unavailable = true;
    }
    update();
  });
  if (canvas) new ResizeObserver(onResize).observe(canvas);
  update();
}

export function getSkyState(): SkyState {
  return current;
}

export function getLiveSkyState(): SkyState {
  return live;
}

export function applySkyState(state: SkyState): void {
  if (!container) return;
  current = state;
  for (const [name, value] of Object.entries(skyGradientVars(state.scene))) {
    container.style.setProperty(name, value);
  }
  const hero = document.querySelector<HTMLElement>(".sky-hero");
  if (hero)
    hero.dataset.skyFg = prefersLightForeground(state.scene) ? "light" : "dark";
  const rates = motionRates(state);
  const t = seconds();
  clocks.drift.retarget(rates.drift, t);
  clocks.rain.retarget([rates.rain, 0], t);
  clocks.snow.retarget(rates.snow, t);
  lastSky = Number.NEGATIVE_INFINITY;
  update();
}

export function setSkyEnabled(on: boolean): void {
  const root = document.documentElement;
  if (on) delete root.dataset.skyEffects;
  else root.dataset.skyEffects = "off";
  try {
    if (on) localStorage.removeItem(STORAGE_KEY);
    else localStorage.setItem(STORAGE_KEY, "off");
  } catch {}
  update();
}
