/**
 * FMI SmartSymbol weather codes, as returned in the `symbol` field of hourly
 * and daily forecasts. Codes >= 100 are night variants of `code - 100`.
 *
 * Labels and the night-asset fallback mirror the official FMI weather app.
 */

export type SymbolTheme = "light" | "dark";

export const SYMBOL_DESCRIPTIONS: Readonly<Record<number, string>> = {
  1: "Clear",
  2: "Mostly clear",
  4: "Partly cloudy",
  6: "Mostly cloudy",
  7: "Cloudy",
  9: "Fog",
  11: "Drizzle",
  14: "Freezing drizzle",
  17: "Freezing rain",
  21: "Isolated showers",
  24: "Scattered showers",
  27: "Showers",
  31: "Isolated light showers",
  32: "Isolated moderate showers",
  33: "Isolated heavy showers",
  34: "Scattered light showers",
  35: "Scattered moderate showers",
  36: "Scattered heavy showers",
  37: "Light rain",
  38: "Moderate rain",
  39: "Heavy rain",
  41: "Isolated light sleet showers",
  42: "Isolated moderate sleet showers",
  43: "Isolated heavy sleet showers",
  44: "Scattered light sleet showers",
  45: "Scattered moderate sleet showers",
  46: "Scattered heavy sleet showers",
  47: "Light sleet",
  48: "Moderate sleet",
  49: "Heavy sleet",
  51: "Isolated light snow showers",
  52: "Isolated moderate snow showers",
  53: "Isolated heavy snow showers",
  54: "Scattered light snow showers",
  55: "Scattered moderate snow showers",
  56: "Scattered heavy snow showers",
  57: "Light snowfall",
  58: "Moderate snowfall",
  59: "Heavy snowfall",
  61: "Isolated hail showers",
  64: "Scattered hail showers",
  67: "Hail showers",
  71: "Isolated thundershowers",
  74: "Scattered thundershowers",
  77: "Thundershowers",
};

/** Codes that have dedicated night artwork (`src/assets/symbols/<theme>/1xx.svg`). */
const NIGHT_ASSET_CODES: ReadonlySet<number> = new Set([
  1, 2, 4, 6, 21, 24, 31, 32, 33, 34, 35, 36, 41, 42, 43, 44, 45, 46, 51, 52,
  53, 54, 55, 56, 61, 64, 71, 74,
]);

export type ParsedSymbol = {
  /** Day code (night offset removed). */
  code: number;
  night: boolean;
};

export function parseSymbolCode(raw: string | null | undefined): ParsedSymbol | null {
  if (raw == null) return null;
  const value = Number.parseInt(raw, 10);
  if (!Number.isFinite(value) || value < 0) return null;
  const night = value >= 100;
  const code = night ? value - 100 : value;
  if (!(code in SYMBOL_DESCRIPTIONS)) return null;
  return { code, night };
}

export function symbolDescription(raw: string | null | undefined): string | null {
  const parsed = parseSymbolCode(raw);
  return parsed ? SYMBOL_DESCRIPTIONS[parsed.code] : null;
}

/**
 * Relative file under `src/assets/symbols/` for the given code and theme,
 * falling back to the day artwork for night codes without a night icon.
 */
export function symbolAssetFile(
  raw: string | null | undefined,
  theme: SymbolTheme,
): string | null {
  const parsed = parseSymbolCode(raw);
  if (!parsed) return null;
  const file =
    parsed.night && NIGHT_ASSET_CODES.has(parsed.code) ? parsed.code + 100 : parsed.code;
  return `${theme}/${file}.svg`;
}
