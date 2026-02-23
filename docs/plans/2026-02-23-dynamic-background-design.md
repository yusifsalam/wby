# Dynamic Atmospheric Background — Design

**Date:** 2026-02-23
**Status:** Approved

## Overview

Replace the hardcoded static `LinearGradient` in `ContentView.mainBackground` with a layered dynamic background: a SwiftUI gradient that responds to weather conditions + time of day, with a SpriteKit particle layer for precipitation, stars, and clouds.

## Weather Scenes

Seven scenes derived from the current FMI symbol code and day/night flag:

| Scene | Trigger | Sky Gradient |
|-------|---------|-------------|
| `clearDay` | code 1, day | bright sky blue → royal blue |
| `clearNight` | code 1, night | deep navy → indigo/near-black |
| `partlyCloudy` | codes 2/4/6/7, day | muted blue → slate blue |
| `partlyCloudyNight` | codes 2/4/6/7, night | dark slate blue → near-black |
| `overcast` | code 7 heavy / fog (9) / sleet (14/17/41–49) | blue-grey → slate |
| `rain` | codes 11/21/24/27/31–39 | dark slate → blue-grey |
| `snow` | codes 51–59 | pale blue → white-grey |
| `storm` | codes 71/74/77 | near-black → dark charcoal |

Day/night is read directly from the symbol code: codes ≥ 100 are night variants (subtract 100 to normalize, as per existing `SmartSymbol` logic).

## Architecture

Three new files under `ios/wby/wby/Background/`:

```
WeatherScene.swift      — enum + gradient palette per scene
WeatherSKScene.swift    — SKScene subclass, owns particle emitters
WeatherSceneView.swift  — SpriteView wrapper View (transparent background)
```

`ContentView.mainBackground` becomes:

```swift
ZStack {
    skyGradient(for: currentScene)   // SwiftUI LinearGradient, animates on change
    WeatherSceneView(scene: currentScene)  // SpriteKit particles on top
}
.ignoresSafeArea()
```

`currentScene` is a computed property on `ContentView` derived from
`weather?.hourlyForecast.first?.symbol ?? String(Int(weather?.current.weatherCode ?? 1))`.

## Particles Per Scene

| Scene | Particles |
|-------|----------|
| `clearDay` | none |
| `clearNight` | 120 star dots, looping twinkle (alpha pulse) |
| `partlyCloudy` | 2–3 slow-drifting blurred white ellipses |
| `partlyCloudyNight` | stars (60) + 1–2 drifting cloud shapes |
| `overcast` | none |
| `rain` | `SKEmitterNode` rain streaks, ~250 particles, slight angle |
| `snow` | `SKEmitterNode` snowflakes, ~150 particles, gentle drift |
| `storm` | rain at 2× density + random lightning flash (SKAction) |

## Sky Gradient Palettes

```swift
clearDay:           [#60BDFF, #3A8FEF, #1C55BB]
clearNight:         [#0D1B4B, #060D26, #020510]
partlyCloudy:       [#5A9FD4, #4178A8, #2D5A8A]
partlyCloudyNight:  [#1A2744, #0F1A33, #080F20]
overcast:           [#6B7F99, #4A5E75, #2E3D52]
rain:               [#3D5066, #2A3A4D, #1A2535]
snow:               [#B8C8D8, #8FA8BC, #6A8AA0]
storm:              [#1A1E2E, #0F1220, #080B15]
```

## Transitions

When `currentScene` changes (weather loads or refreshes), the gradient animates via `withAnimation(.easeInOut(duration: 1.5))`. The `WeatherSKScene` transitions particles by removing old emitters and adding new ones with a short fade.

## SpriteKit Scene Setup

- `SKScene.backgroundColor = .clear` — gradient shows through
- Particle emitters positioned at `CGPoint(x: frame.midX, y: frame.maxY + 20)` for rain/snow (emits upward, gravity pulls down), or scattered across the sky for stars
- `preferredFramesPerSecond = 30` — sufficient for weather particles, saves battery
- Scene size matches `UIScreen.main.bounds` and is set once in `WeatherSceneView.onAppear`

## Files to Create

- `ios/wby/wby/Background/WeatherScene.swift`
- `ios/wby/wby/Background/WeatherSKScene.swift`
- `ios/wby/wby/Background/WeatherSceneView.swift`

## Files to Modify

- `ios/wby/wby/ContentView.swift` — replace `mainBackground`, add `currentScene` computed property
