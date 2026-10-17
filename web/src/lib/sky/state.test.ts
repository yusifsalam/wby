import { describe, expect, it } from "vitest";
import type { WeatherResponse } from "../weatherApi";
import {
  coverage,
  cssRgb,
  motionRates,
  precipitationIntensity,
  rainSlant,
  type SkyState,
  sceneFromSymbol,
  skyStateFromWeather,
  wind,
} from "./state";

const state = (values: Partial<SkyState> = {}): SkyState => ({
  scene: "rain",
  precipitation1h: null,
  cloudCover: null,
  windSpeed: null,
  windDirection: null,
  ...values,
});

describe("sky scene", () => {
  it("maps SmartSymbol codes like the iOS app", () => {
    expect(sceneFromSymbol("1")).toBe("clearDay");
    expect(sceneFromSymbol("101")).toBe("clearNight");
    expect(sceneFromSymbol("3")).toBe("partlyCloudy");
    expect(sceneFromSymbol("106")).toBe("partlyCloudyNight");
    expect(sceneFromSymbol("5")).toBe("overcast");
    expect(sceneFromSymbol("9")).toBe("overcast");
    expect(sceneFromSymbol("17")).toBe("overcast");
    expect(sceneFromSymbol("148")).toBe("overcast");
    expect(sceneFromSymbol("11")).toBe("rain");
    expect(sceneFromSymbol("138")).toBe("rain");
    expect(sceneFromSymbol("59")).toBe("snow");
    expect(sceneFromSymbol("67")).toBe("snow");
    expect(sceneFromSymbol("177")).toBe("storm");
  });

  it("falls back to clear sky for unknown or missing codes", () => {
    expect(sceneFromSymbol("99")).toBe("clearDay");
    expect(sceneFromSymbol("199")).toBe("clearNight");
    expect(sceneFromSymbol("abc")).toBe("clearDay");
    expect(sceneFromSymbol(null)).toBe("clearDay");
    expect(sceneFromSymbol(undefined)).toBe("clearDay");
  });
});

describe("sky state", () => {
  it("normalizes cloud cover from oktas or percent", () => {
    expect(coverage(state({ cloudCover: 4 }))).toBe(0.5);
    expect(coverage(state({ cloudCover: 8 }))).toBe(1);
    expect(coverage(state({ cloudCover: 75 }))).toBe(0.75);
    expect(coverage(state({ cloudCover: 120 }))).toBe(1);
    expect(coverage(state())).toBe(0.5);
  });

  it("derives precipitation intensity with per-scene floors", () => {
    expect(precipitationIntensity(state())).toBe(0.9);
    expect(precipitationIntensity(state({ precipitation1h: 2 }))).toBe(1);
    expect(precipitationIntensity(state({ precipitation1h: 0.001 }))).toBe(
      0.15,
    );
    expect(precipitationIntensity(state({ precipitation1h: 50 }))).toBe(3);
    expect(precipitationIntensity(state({ scene: "storm" }))).toBe(2.4);
    expect(
      precipitationIntensity(state({ scene: "storm", precipitation1h: 0.5 })),
    ).toBe(2);
  });

  it("points the wind vector where the wind blows to", () => {
    const [x, y] = wind(state({ windSpeed: 10, windDirection: 270 }));
    expect(x).toBeCloseTo(10);
    expect(y).toBeCloseTo(0);
    const [nx, ny] = wind(state({ windSpeed: 5, windDirection: 0 }));
    expect(nx).toBeCloseTo(0);
    expect(ny).toBeCloseTo(-5);
    expect(rainSlant(state({ windSpeed: 4, windDirection: 270 }))).toBeCloseTo(
      0.12,
    );
    expect(rainSlant(state({ windSpeed: 30, windDirection: 90 }))).toBe(-0.35);
  });

  it("scales motion rates with wind and intensity", () => {
    const calm = motionRates(state({ windSpeed: 0 }));
    expect(calm.drift).toEqual([1.5 * 0.0072, 0]);
    expect(calm.snow).toEqual([0, 120]);
    const heavy = motionRates(
      state({ precipitation1h: 10, windSpeed: 20, windDirection: 270 }),
    );
    expect(heavy.rain).toBe(740);
    expect(heavy.drift[0]).toBeCloseTo(15 * 0.0072);
    expect(heavy.snow[0]).toBe(70);
  });

  it("builds the state from the weather response", () => {
    const data = {
      current: {
        observed_at: "",
        cloud_cover: 6,
        wind_speed: null,
        wind_direction: 200,
      },
      hourly_forecast: [
        {
          time: "",
          symbol: "137",
          precipitation_1h: 0.4,
          wind_speed: 5,
          wind_direction: 90,
        },
      ],
      daily_forecast: [{ date: "", symbol: "1" }],
    } as unknown as WeatherResponse;
    expect(skyStateFromWeather(data)).toEqual({
      scene: "rain",
      precipitation1h: 0.4,
      cloudCover: 6,
      windSpeed: 5,
      windDirection: 200,
    });
  });

  it("formats colors as exact sRGB percentages", () => {
    expect(cssRgb([0.38, 0.07, 1])).toBe("rgb(38% 7% 100%)");
  });
});
