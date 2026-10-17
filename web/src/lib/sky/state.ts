/**
 * Weather background state, ported from the iOS app
 * (ios/wby/wby/Background/WeatherScene.swift and WeatherBackgroundState.swift).
 */
import type { WeatherResponse } from "../weatherApi";

export const SKY_SCENES = [
  "clearDay",
  "clearNight",
  "partlyCloudy",
  "partlyCloudyNight",
  "overcast",
  "rain",
  "snow",
  "storm",
] as const;
export type SkyScene = (typeof SKY_SCENES)[number];

export type Rgb = readonly [number, number, number];
export type Vec2 = [number, number];

export type SkyState = {
  scene: SkyScene;
  precipitation1h: number | null;
  cloudCover: number | null;
  windSpeed: number | null;
  windDirection: number | null;
};

export type SkyStyle = {
  cumulus: number;
  deck: number;
  cirrus: number;
  absorption: number;
  sun: number;
  stars: number;
  night: number;
  lit: Rgb;
  shadow: Rgb;
};

export type MotionRates = {
  drift: Vec2;
  rain: number;
  snow: Vec2;
};

export function isSkyScene(value: unknown): value is SkyScene {
  return (
    typeof value === "string" &&
    (SKY_SCENES as readonly string[]).includes(value)
  );
}

export function sceneFromSymbol(raw: string | null | undefined): SkyScene {
  if (raw == null || !/^\d+$/.test(raw)) return "clearDay";
  const code = Number(raw);
  const night = code >= 100;
  switch (night ? code - 100 : code) {
    case 1:
      return night ? "clearNight" : "clearDay";
    case 2:
    case 3:
    case 4:
    case 6:
      return night ? "partlyCloudyNight" : "partlyCloudy";
    case 5:
    case 7:
    case 9:
    case 14:
    case 17:
    case 41:
    case 42:
    case 43:
    case 44:
    case 45:
    case 46:
    case 47:
    case 48:
    case 49:
      return "overcast";
    case 11:
    case 21:
    case 24:
    case 27:
    case 31:
    case 32:
    case 33:
    case 34:
    case 35:
    case 36:
    case 37:
    case 38:
    case 39:
      return "rain";
    case 51:
    case 52:
    case 53:
    case 54:
    case 55:
    case 56:
    case 57:
    case 58:
    case 59:
    case 61:
    case 64:
    case 67:
      return "snow";
    case 71:
    case 74:
    case 77:
      return "storm";
    default:
      return night ? "clearNight" : "clearDay";
  }
}

export const SKY_GRADIENTS: Readonly<
  Record<SkyScene, readonly [Rgb, Rgb, Rgb]>
> = {
  clearDay: [
    [0.38, 0.74, 0.99],
    [0.23, 0.54, 0.94],
    [0.11, 0.33, 0.73],
  ],
  clearNight: [
    [0.05, 0.11, 0.3],
    [0.02, 0.05, 0.15],
    [0.01, 0.02, 0.08],
  ],
  partlyCloudy: [
    [0.35, 0.62, 0.83],
    [0.26, 0.47, 0.67],
    [0.18, 0.36, 0.54],
  ],
  partlyCloudyNight: [
    [0.1, 0.15, 0.27],
    [0.06, 0.1, 0.2],
    [0.03, 0.05, 0.13],
  ],
  overcast: [
    [0.42, 0.5, 0.6],
    [0.29, 0.37, 0.46],
    [0.18, 0.24, 0.32],
  ],
  rain: [
    [0.24, 0.31, 0.41],
    [0.17, 0.23, 0.3],
    [0.1, 0.15, 0.21],
  ],
  snow: [
    [0.72, 0.79, 0.85],
    [0.56, 0.67, 0.74],
    [0.42, 0.54, 0.63],
  ],
  storm: [
    [0.1, 0.12, 0.18],
    [0.06, 0.07, 0.13],
    [0.03, 0.04, 0.09],
  ],
};

export function cssRgb([r, g, b]: Rgb): string {
  const pct = (v: number) => `${Math.round(v * 1000) / 10}%`;
  return `rgb(${pct(r)} ${pct(g)} ${pct(b)})`;
}

export function skyGradientVars(scene: SkyScene): Record<string, string> {
  const [top, mid, bottom] = SKY_GRADIENTS[scene];
  return {
    "--sky-top": cssRgb(top),
    "--sky-mid": cssRgb(mid),
    "--sky-bottom": cssRgb(bottom),
  };
}

export function prefersLightForeground(scene: SkyScene): boolean {
  return (
    scene === "clearNight" ||
    scene === "partlyCloudyNight" ||
    scene === "overcast" ||
    scene === "rain" ||
    scene === "storm"
  );
}

export function hasRain(scene: SkyScene): boolean {
  return scene === "rain" || scene === "storm";
}

export function hasPrecipitation(scene: SkyScene): boolean {
  return hasRain(scene) || scene === "snow";
}

export function skyStyle(scene: SkyScene, coverage: number): SkyStyle {
  const base = { deck: 0, cirrus: 0, sun: 0, stars: 0, night: 0 };
  switch (scene) {
    case "clearDay":
      return {
        ...base,
        cumulus: coverage * 0.12,
        cirrus: 0.3,
        absorption: 0.3,
        sun: 1,
        lit: [1.0, 0.99, 0.97],
        shadow: [0.7, 0.79, 0.92],
      };
    case "clearNight":
      return {
        ...base,
        cumulus: coverage * 0.12,
        cirrus: 0.15,
        absorption: 0.4,
        stars: 1,
        night: 1,
        lit: [0.3, 0.33, 0.42],
        shadow: [0.1, 0.12, 0.21],
      };
    case "partlyCloudy":
      return {
        ...base,
        cumulus: 0.25 + coverage * 0.55,
        cirrus: 0.4,
        absorption: 0.45,
        sun: 1,
        lit: [1.0, 0.99, 0.96],
        shadow: [0.6, 0.68, 0.8],
      };
    case "partlyCloudyNight":
      return {
        ...base,
        cumulus: 0.25 + coverage * 0.55,
        cirrus: 0.2,
        absorption: 0.5,
        stars: 1,
        night: 1,
        lit: [0.34, 0.37, 0.46],
        shadow: [0.12, 0.15, 0.24],
      };
    case "overcast":
      return {
        ...base,
        cumulus: 1,
        deck: 1,
        absorption: 0.6,
        lit: [0.52, 0.57, 0.64],
        shadow: [0.3, 0.35, 0.43],
      };
    case "rain":
      return {
        ...base,
        cumulus: 1,
        deck: 1,
        absorption: 0.9,
        lit: [0.42, 0.47, 0.54],
        shadow: [0.18, 0.22, 0.29],
      };
    case "snow":
      return {
        ...base,
        cumulus: 1,
        deck: 1,
        absorption: 0.35,
        lit: [0.9, 0.92, 0.95],
        shadow: [0.64, 0.7, 0.78],
      };
    case "storm":
      return {
        ...base,
        cumulus: 1,
        deck: 1,
        absorption: 1.3,
        lit: [0.28, 0.3, 0.38],
        shadow: [0.07, 0.08, 0.12],
      };
  }
}

const clamp = (value: number, min: number, max: number) =>
  Math.min(Math.max(value, min), max);

export function coverage(state: SkyState): number {
  const cover = state.cloudCover;
  if (cover == null) return 0.5;
  return clamp(cover <= 8.5 ? cover / 8 : cover / 100, 0, 1);
}

export function precipitationIntensity(state: SkyState): number {
  const storm = state.scene === "storm";
  const base = storm ? 2.4 : 0.9;
  const floor = storm ? 2.0 : 0.15;
  const mm = state.precipitation1h;
  if (mm == null || mm <= 0) return base;
  return clamp(Math.sqrt(mm / 2), floor, 3);
}

export function wind(state: SkyState): Vec2 {
  const speed = state.windSpeed ?? 3;
  const radians = ((state.windDirection ?? 270) * Math.PI) / 180;
  return [-Math.sin(radians) * speed, -Math.cos(radians) * speed];
}

export function rainSlant(state: SkyState): number {
  return clamp(wind(state)[0] * 0.03, -0.35, 0.35);
}

export function motionRates(state: SkyState): MotionRates {
  const [wx, wy] = wind(state);
  const speed = Math.hypot(wx, wy);
  const direction: Vec2 = speed > 0.01 ? [wx / speed, wy / speed] : [1, 0];
  const drift = clamp(speed, 1.5, 15) * 0.0072;
  const w = clamp(precipitationIntensity(state) / 1.8, 0, 1);
  return {
    drift: [direction[0] * drift, direction[1] * drift],
    rain: 280 + (740 - 280) * w,
    snow: [clamp(wx * 8, -70, 70), 120],
  };
}

export function skyStateFromWeather(data: WeatherResponse): SkyState {
  const nextHour = data.hourly_forecast[0];
  return {
    scene: sceneFromSymbol(nextHour?.symbol ?? data.daily_forecast[0]?.symbol),
    precipitation1h: nextHour?.precipitation_1h ?? null,
    cloudCover: data.current.cloud_cover ?? null,
    windSpeed: data.current.wind_speed ?? nextHour?.wind_speed ?? null,
    windDirection:
      data.current.wind_direction ?? nextHour?.wind_direction ?? null,
  };
}

function preset(
  scene: SkyScene,
  values: Partial<Omit<SkyState, "scene">> = {},
): SkyState {
  return {
    scene,
    precipitation1h: null,
    cloudCover: null,
    windSpeed: null,
    windDirection: null,
    ...values,
  };
}

export const SKY_PRESETS: readonly { label: string; state: SkyState }[] = [
  { label: "Clear day", state: preset("clearDay", { cloudCover: 1 }) },
  { label: "Clear night", state: preset("clearNight") },
  {
    label: "Partly cloudy",
    state: preset("partlyCloudy", {
      cloudCover: 5,
      windSpeed: 6,
      windDirection: 250,
    }),
  },
  {
    label: "Partly cloudy night",
    state: preset("partlyCloudyNight", { cloudCover: 5 }),
  },
  {
    label: "Overcast",
    state: preset("overcast", {
      cloudCover: 8,
      windSpeed: 4,
      windDirection: 180,
    }),
  },
  {
    label: "Rain",
    state: preset("rain", {
      precipitation1h: 0.6,
      cloudCover: 8,
      windSpeed: 7,
      windDirection: 240,
    }),
  },
  {
    label: "Snow",
    state: preset("snow", {
      precipitation1h: 0.8,
      cloudCover: 8,
      windSpeed: 4,
      windDirection: 300,
    }),
  },
  {
    label: "Storm",
    state: preset("storm", {
      precipitation1h: 3,
      cloudCover: 8,
      windSpeed: 12,
      windDirection: 220,
    }),
  },
];
