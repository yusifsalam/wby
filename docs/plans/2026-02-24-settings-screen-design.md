# Settings Screen Design

**Date:** 2026-02-24

## Overview

Add a settings screen to the iOS app where the user can toggle dynamic background particle effects on or off. The setting persists across launches via `UserDefaults` (`@AppStorage`).

## Scope

- Single toggle: **Dynamic Background Effects** (controls SpriteKit particle animations)
- The weather-reactive gradient always remains — only particles are affected
- Accessed via a gear icon in the main screen toolbar
- Presented as a pushed navigation screen (not a sheet)

## Architecture

### Persistence

`@AppStorage("dynamicEffectsEnabled")` in both `ContentView` and `SettingsView`, both reading/writing the same `UserDefaults` key. Default value: `true`. No binding passing needed.

### Navigation

`ContentView.body` is wrapped in a `NavigationStack`. The gear toolbar button uses a `NavigationLink` to push `SettingsView`. Navigation bar is styled transparently so the weather gradient shows through.

### SettingsView

New file: `ios/wby/wby/Views/SettingsView.swift`

A `Form` containing a single `Toggle("Dynamic Background Effects", isOn: $dynamicEffectsEnabled)` bound to `@AppStorage("dynamicEffectsEnabled")`.

### Effect on background

In `ContentView.mainBackground`, `WeatherSceneView(...)` is wrapped in `if dynamicEffectsEnabled { ... }`. When disabled, only the `LinearGradient` renders.

## Files Changed

- `ios/wby/wby/ContentView.swift` — add `NavigationStack`, `@AppStorage`, toolbar gear button, conditional `WeatherSceneView`
- `ios/wby/wby/Views/SettingsView.swift` — new file
