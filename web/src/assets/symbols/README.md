# Weather symbol icons

SVG weather symbols from the official FMI weather app assets repository:
https://github.com/fmidev/weather-app-assets (`images/symbols/`, commit `c49bd21`).

- File name = FMI SmartSymbol code (`1.svg` … `77.svg`); `1xx` files are the
  night variants, which exist only for codes where the sun/moon is visible.
  Overcast and continuous-precipitation codes reuse the day icon.
- `light/` is for the light UI theme, `dark/` for the dark theme (same
  geometry, recoloured).

Code → label mapping and the night fallback live in `src/lib/weatherSymbols.ts`;
`src/components/WeatherSymbol.astro` loads the files via `import.meta.glob` and
renders them with `<Image>` from `astro:assets` (hashed, cacheable URLs).
