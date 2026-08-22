import { describe, expect, it } from "vitest";
import {
  SYMBOL_DESCRIPTIONS,
  parseSymbolCode,
  symbolAssetFile,
  symbolDescription,
} from "./weatherSymbols";

describe("weather symbols", () => {
  it("parses day and night SmartSymbol codes", () => {
    expect(parseSymbolCode("1")).toEqual({ code: 1, night: false });
    expect(parseSymbolCode("138")).toEqual({ code: 38, night: true });
    expect(parseSymbolCode("3")).toBeNull();
    expect(parseSymbolCode("abc")).toBeNull();
    expect(parseSymbolCode(null)).toBeNull();
    expect(parseSymbolCode(undefined)).toBeNull();
  });

  it("describes codes with the night offset stripped", () => {
    expect(symbolDescription("7")).toBe("Cloudy");
    expect(symbolDescription("177")).toBe("Thundershowers");
    expect(symbolDescription("99")).toBeNull();
  });

  it("resolves asset files with night artwork only where it exists", () => {
    expect(symbolAssetFile("1", "light")).toBe("light/1.svg");
    expect(symbolAssetFile("101", "dark")).toBe("dark/101.svg");
    // Cloudy has no separate night icon: fall back to the day file.
    expect(symbolAssetFile("107", "light")).toBe("light/7.svg");
    expect(symbolAssetFile("138", "light")).toBe("light/38.svg");
    expect(symbolAssetFile("3", "light")).toBeNull();
  });

  it("covers all 45 day codes", () => {
    expect(Object.keys(SYMBOL_DESCRIPTIONS)).toHaveLength(45);
  });
});
