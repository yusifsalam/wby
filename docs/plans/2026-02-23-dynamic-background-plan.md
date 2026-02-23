# Dynamic Atmospheric Background — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the static blue gradient in `ContentView` with a weather-responsive SpriteKit particle scene: animated rain, snow, stars, and drifting clouds driven by the FMI symbol code.

**Architecture:** A `WeatherScene` enum maps FMI symbol codes to one of 8 scene types. A `WeatherSKScene: SKScene` owns SpriteKit particle emitters for each scene type. A `WeatherSceneView` wraps it in a `SpriteView`. `ContentView.mainBackground` becomes a `ZStack` of an animating SwiftUI `LinearGradient` (sky colour) and the transparent `SpriteView` (particles) on top.

**Tech Stack:** SwiftUI, SpriteKit (`import SpriteKit`), `UIGraphicsImageRenderer` for programmatic textures. No `.sks` files, no third-party dependencies.

---

## Key Paths

- New: `ios/wby/wby/Background/WeatherScene.swift`
- New: `ios/wby/wby/Background/WeatherSKScene.swift`
- New: `ios/wby/wby/Background/WeatherSceneView.swift`
- Modify: `ios/wby/wby/ContentView.swift`

## FMI Symbol Code Reference

Symbol codes come from `HourlyForecast.symbol` (String) and `DailyForecast.symbol` (String). Codes ≥ 100 are night variants (e.g. 101 = clear night). Normalize by subtracting 100 for code ≥ 100 — the day/night distinction is preserved as a boolean.

```
1          → clear
2, 4, 6    → partly cloudy
7          → cloudy / overcast
9          → fog → overcast
11         → light drizzle → rain
14, 17     → sleet → overcast
21, 24, 27 → rain (light/moderate/heavy)
31–39      → shower rain
41–49      → sleet showers → overcast
51–59      → snow
61, 64, 67 → hail → rain
71, 74, 77 → thunderstorm → storm
```

---

## Task 1: WeatherScene enum

**Files:**
- Create: `ios/wby/wby/Background/WeatherScene.swift`

**Step 1: Create the file**

```swift
import SwiftUI

enum WeatherScene: Equatable {
    case clearDay
    case clearNight
    case partlyCloudy
    case partlyCloudyNight
    case overcast
    case rain
    case snow
    case storm

    static func from(symbolCode: String?) -> WeatherScene {
        guard let code = symbolCode.flatMap(Int.init) else { return .clearDay }
        let isNight = code >= 100
        let normalized = isNight ? code - 100 : code

        switch normalized {
        case 1:
            return isNight ? .clearNight : .clearDay
        case 2, 4, 6:
            return isNight ? .partlyCloudyNight : .partlyCloudy
        case 7, 9:
            return .overcast
        case 14, 17, 41, 42, 43, 44, 45, 46, 47, 48, 49:
            return .overcast
        case 11, 21, 24, 27, 31, 32, 33, 34, 35, 36, 37, 38, 39:
            return .rain
        case 51, 52, 53, 54, 55, 56, 57, 58, 59:
            return .snow
        case 61, 64, 67:
            return .rain
        case 71, 74, 77:
            return .storm
        default:
            return isNight ? .clearNight : .clearDay
        }
    }

    var gradientColors: [Color] {
        switch self {
        case .clearDay:
            return [
                Color(red: 0.38, green: 0.74, blue: 0.99),
                Color(red: 0.23, green: 0.54, blue: 0.94),
                Color(red: 0.11, green: 0.33, blue: 0.73),
            ]
        case .clearNight:
            return [
                Color(red: 0.05, green: 0.11, blue: 0.30),
                Color(red: 0.02, green: 0.05, blue: 0.15),
                Color(red: 0.01, green: 0.02, blue: 0.08),
            ]
        case .partlyCloudy:
            return [
                Color(red: 0.35, green: 0.62, blue: 0.83),
                Color(red: 0.26, green: 0.47, blue: 0.67),
                Color(red: 0.18, green: 0.36, blue: 0.54),
            ]
        case .partlyCloudyNight:
            return [
                Color(red: 0.10, green: 0.15, blue: 0.27),
                Color(red: 0.06, green: 0.10, blue: 0.20),
                Color(red: 0.03, green: 0.05, blue: 0.13),
            ]
        case .overcast:
            return [
                Color(red: 0.42, green: 0.50, blue: 0.60),
                Color(red: 0.29, green: 0.37, blue: 0.46),
                Color(red: 0.18, green: 0.24, blue: 0.32),
            ]
        case .rain:
            return [
                Color(red: 0.24, green: 0.31, blue: 0.41),
                Color(red: 0.17, green: 0.23, blue: 0.30),
                Color(red: 0.10, green: 0.15, blue: 0.21),
            ]
        case .snow:
            return [
                Color(red: 0.72, green: 0.79, blue: 0.85),
                Color(red: 0.56, green: 0.67, blue: 0.74),
                Color(red: 0.42, green: 0.54, blue: 0.63),
            ]
        case .storm:
            return [
                Color(red: 0.10, green: 0.12, blue: 0.18),
                Color(red: 0.06, green: 0.07, blue: 0.13),
                Color(red: 0.03, green: 0.04, blue: 0.09),
            ]
        }
    }
}
```

**Step 2: Verify it builds**

```
xcodebuild -project ios/wby/wby.xcodeproj -scheme wby \
  -destination 'generic/platform=iOS Simulator' build 2>&1 | tail -5
```
Expected: `** BUILD SUCCEEDED **`

**Step 3: Commit**

```bash
git add ios/wby/wby/Background/WeatherScene.swift
git commit -m "ios: add WeatherScene enum with gradient palettes"
```

---

## Task 2: WeatherSKScene

**Files:**
- Create: `ios/wby/wby/Background/WeatherSKScene.swift`

SpriteKit coordinate system: origin at **bottom-left**, y increases upward. Top of screen = `frame.maxY`. Emitters for rain/snow are placed at `(midX, maxY + 20)` so particles enter from above.

**Step 1: Create the file**

```swift
import SpriteKit

final class WeatherSKScene: SKScene {

    private var currentWeatherScene: WeatherScene

    init(size: CGSize, weatherScene: WeatherScene) {
        self.currentWeatherScene = weatherScene
        super.init(size: size)
        backgroundColor = .clear
        scaleMode = .resizeFill
    }

    required init?(coder aDecoder: NSCoder) {
        fatalError("init(coder:) not supported")
    }

    override func didMove(to view: SKView) {
        view.allowsTransparency = true
        setupParticles(for: currentWeatherScene)
    }

    func transition(to newScene: WeatherScene) {
        guard newScene != currentWeatherScene else { return }
        currentWeatherScene = newScene
        removeAllChildren()
        removeAllActions()
        setupParticles(for: newScene)
    }

    // MARK: - Scene Setup

    private func setupParticles(for scene: WeatherScene) {
        switch scene {
        case .clearDay, .overcast:
            break
        case .clearNight:
            addStars(count: 120)
        case .partlyCloudy:
            addClouds(count: 3)
        case .partlyCloudyNight:
            addStars(count: 60)
            addClouds(count: 2)
        case .rain:
            addRain(intensity: 1.0)
        case .snow:
            addSnow()
        case .storm:
            addRain(intensity: 2.0)
            scheduleLightning()
        }
    }

    // MARK: - Stars

    private func addStars(count: Int) {
        for _ in 0..<count {
            let star = SKSpriteNode(color: .white, size: CGSize(width: 2, height: 2))
            star.position = CGPoint(
                x: CGFloat.random(in: 0...size.width),
                y: CGFloat.random(in: size.height * 0.25...size.height)
            )
            star.alpha = CGFloat.random(in: 0.3...0.9)
            addChild(star)

            let minAlpha = CGFloat.random(in: 0.1...0.3)
            let maxAlpha = CGFloat.random(in: 0.6...1.0)
            let duration = TimeInterval.random(in: 1.0...3.0)
            let fadeOut = SKAction.fadeAlpha(to: minAlpha, duration: duration)
            let fadeIn = SKAction.fadeAlpha(to: maxAlpha, duration: duration)
            star.run(.repeatForever(.sequence([fadeOut, fadeIn])))
        }
    }

    // MARK: - Clouds

    private func addClouds(count: Int) {
        for i in 0..<count {
            let cloud = makeCloudNode()
            let xStart = -200 + CGFloat(i) * (size.width / CGFloat(count))
            let yPos = size.height * CGFloat.random(in: 0.60...0.88)
            cloud.position = CGPoint(x: xStart, y: yPos)
            addChild(cloud)

            let travelDuration = TimeInterval.random(in: 40...80)
            let moveAcross = SKAction.moveBy(x: size.width + 400, y: 0, duration: travelDuration)
            let resetPos = SKAction.run { [weak cloud, weak self] in
                guard let cloud, let self else { return }
                cloud.position.x = -200
                cloud.position.y = self.size.height * CGFloat.random(in: 0.60...0.88)
            }
            cloud.run(.repeatForever(.sequence([moveAcross, resetPos])))
        }
    }

    private func makeCloudNode() -> SKNode {
        let container = SKNode()
        let puffs: [(CGFloat, CGFloat, CGFloat)] = [
            (0, 0, 45),
            (-40, -10, 32),
            (40, -10, 36),
            (-20, 12, 26),
            (22, 14, 28),
        ]
        for (x, y, radius) in puffs {
            let puff = SKShapeNode(circleOfRadius: radius)
            puff.fillColor = UIColor.white.withAlphaComponent(0.11)
            puff.strokeColor = .clear
            puff.position = CGPoint(x: x, y: y)
            container.addChild(puff)
        }
        return container
    }

    // MARK: - Rain

    private func addRain(intensity: CGFloat) {
        let emitter = makeRainEmitter(intensity: intensity)
        emitter.position = CGPoint(x: size.width / 2, y: size.height + 20)
        emitter.particlePositionRange = CGVector(dx: size.width * 1.5, dy: 0)
        addChild(emitter)
    }

    private func makeRainEmitter(intensity: CGFloat) -> SKEmitterNode {
        let emitter = SKEmitterNode()
        emitter.particleTexture = rainTexture()
        emitter.particleBirthRate = 250 * intensity
        emitter.particleLifetime = 0.8
        emitter.particleLifetimeRange = 0.2
        emitter.particleSpeed = 900
        emitter.particleSpeedRange = 150
        emitter.emissionAngle = -.pi / 2 + 0.15
        emitter.emissionAngleRange = 0.05
        emitter.particleScale = 0.3
        emitter.particleScaleRange = 0.1
        emitter.particleAlpha = 0.35
        emitter.particleAlphaRange = 0.1
        emitter.particleColor = UIColor(white: 0.85, alpha: 1.0)
        emitter.particleColorBlendFactor = 1.0
        emitter.yAcceleration = -200
        emitter.xAcceleration = -80
        return emitter
    }

    private func rainTexture() -> SKTexture {
        let size = CGSize(width: 1, height: 12)
        let renderer = UIGraphicsImageRenderer(size: size)
        let image = renderer.image { ctx in
            UIColor.white.setFill()
            ctx.fill(CGRect(origin: .zero, size: size))
        }
        return SKTexture(image: image)
    }

    // MARK: - Snow

    private func addSnow() {
        let emitter = makeSnowEmitter()
        emitter.position = CGPoint(x: size.width / 2, y: size.height + 20)
        emitter.particlePositionRange = CGVector(dx: size.width * 1.5, dy: 0)
        addChild(emitter)
    }

    private func makeSnowEmitter() -> SKEmitterNode {
        let emitter = SKEmitterNode()
        emitter.particleTexture = snowTexture()
        emitter.particleBirthRate = 60
        emitter.particleLifetime = 5.0
        emitter.particleLifetimeRange = 2.0
        emitter.particleSpeed = 120
        emitter.particleSpeedRange = 40
        emitter.emissionAngle = -.pi / 2
        emitter.emissionAngleRange = 0.3
        emitter.particleScale = 0.15
        emitter.particleScaleRange = 0.08
        emitter.particleAlpha = 0.8
        emitter.particleAlphaRange = 0.2
        emitter.particleColor = .white
        emitter.particleColorBlendFactor = 1.0
        emitter.yAcceleration = -30
        emitter.xAcceleration = CGFloat.random(in: -20...20)
        return emitter
    }

    private func snowTexture() -> SKTexture {
        let size = CGSize(width: 10, height: 10)
        let renderer = UIGraphicsImageRenderer(size: size)
        let image = renderer.image { ctx in
            UIColor.white.setFill()
            ctx.cgContext.fillEllipse(in: CGRect(origin: .zero, size: size))
        }
        return SKTexture(image: image)
    }

    // MARK: - Lightning

    private func scheduleLightning() {
        let wait = SKAction.wait(forDuration: 8, withRange: 10)
        let flash = SKAction.run { [weak self] in self?.flashLightning() }
        run(.repeatForever(.sequence([wait, flash])), withKey: "lightning")
    }

    private func flashLightning() {
        let flash = SKSpriteNode(color: .white, size: size)
        flash.position = CGPoint(x: size.width / 2, y: size.height / 2)
        flash.alpha = 0
        flash.zPosition = 100
        addChild(flash)

        let fadeIn = SKAction.fadeAlpha(to: 0.45, duration: 0.05)
        let hold = SKAction.wait(forDuration: 0.05)
        let fadeOut = SKAction.fadeOut(withDuration: 0.25)
        let remove = SKAction.removeFromParent()
        flash.run(.sequence([fadeIn, hold, fadeOut, remove]))
    }
}
```

**Step 2: Verify it builds**

```
xcodebuild -project ios/wby/wby.xcodeproj -scheme wby \
  -destination 'generic/platform=iOS Simulator' build 2>&1 | tail -5
```
Expected: `** BUILD SUCCEEDED **`

**Step 3: Commit**

```bash
git add ios/wby/wby/Background/WeatherSKScene.swift
git commit -m "ios: add WeatherSKScene with rain, snow, stars, clouds, lightning"
```

---

## Task 3: WeatherSceneView

**Files:**
- Create: `ios/wby/wby/Background/WeatherSceneView.swift`

`SpriteView` needs `options: [.allowsTransparency]` and the scene must have `backgroundColor = .clear` (already set in `WeatherSKScene.init`). The view uses `GeometryReader` to size the scene to the full screen on first appear.

**Step 1: Create the file**

```swift
import SwiftUI
import SpriteKit

struct WeatherSceneView: View {
    let weatherScene: WeatherScene

    @State private var skScene: WeatherSKScene?

    var body: some View {
        GeometryReader { geo in
            Group {
                if let skScene {
                    SpriteView(scene: skScene, options: [.allowsTransparency])
                }
            }
            .onAppear {
                guard skScene == nil else { return }
                skScene = WeatherSKScene(size: geo.size, weatherScene: weatherScene)
            }
            .onChange(of: weatherScene) { _, newScene in
                skScene?.transition(to: newScene)
            }
        }
    }
}
```

**Step 2: Verify it builds**

```
xcodebuild -project ios/wby/wby.xcodeproj -scheme wby \
  -destination 'generic/platform=iOS Simulator' build 2>&1 | tail -5
```
Expected: `** BUILD SUCCEEDED **`

**Step 3: Commit**

```bash
git add ios/wby/wby/Background/WeatherSceneView.swift
git commit -m "ios: add WeatherSceneView SpriteKit wrapper"
```

---

## Task 4: Wire into ContentView

**Files:**
- Modify: `ios/wby/wby/ContentView.swift`

**Step 1: Add `currentScene` computed property**

Add this after the existing private properties (after `let fallbackCoordinate`):

```swift
private var currentScene: WeatherScene {
    let symbol = weather?.hourlyForecast.first?.symbol
        ?? weather?.dailyForecast.first?.symbol
    return WeatherScene.from(symbolCode: symbol)
}
```

Note: `CurrentConditions.weatherCode` is the FMI wawa parameter, not the symbol code — don't use it here. Symbol codes come from forecast data only.

**Step 2: Replace `mainBackground`**

Replace the existing `mainBackground` computed property:

```swift
// BEFORE:
private var mainBackground: some View {
    LinearGradient(
        colors: [
            Color(red: 0.38, green: 0.74, blue: 0.99),
            Color(red: 0.23, green: 0.54, blue: 0.94),
            Color(red: 0.11, green: 0.33, blue: 0.73),
        ],
        startPoint: .top,
        endPoint: .bottom
    )
    .ignoresSafeArea()
}

// AFTER:
private var mainBackground: some View {
    ZStack {
        LinearGradient(
            colors: currentScene.gradientColors,
            startPoint: .top,
            endPoint: .bottom
        )
        .ignoresSafeArea()
        .id(currentScene)
        .transition(.opacity)

        WeatherSceneView(weatherScene: currentScene)
            .ignoresSafeArea()
    }
    .animation(.easeInOut(duration: 1.5), value: currentScene)
}
```

**Step 3: Verify it builds**

```
xcodebuild -project ios/wby/wby.xcodeproj -scheme wby \
  -destination 'generic/platform=iOS Simulator' build 2>&1 | tail -5
```
Expected: `** BUILD SUCCEEDED **`

**Step 4: Commit**

```bash
git add ios/wby/wby/ContentView.swift
git commit -m "ios: wire dynamic atmospheric background into ContentView"
```

---

## Task 5: Preview verification

**Files:**
- Modify: `ios/wby/wby/ContentView.swift` — update preview data symbol to exercise scenes

**Step 1: Test each scene in Xcode previews**

To exercise the snow scene in the preview, temporarily change the first `HourlyForecast` symbol from `"2"` to `"51"` in the preview data at the bottom of `ContentView.swift`. Run the preview. Verify snowflakes fall over a pale blue-grey sky.

Test other scenes by changing to:
- `"1"` → clearDay (bright blue, no particles)
- `"101"` → clearNight (dark sky, twinkling stars)
- `"21"` → rain (dark grey sky, falling rain)
- `"71"` → storm (near-black sky, heavy rain, lightning)
- `"51"` → snow (pale blue-grey, snowflakes)
- `"2"` → partlyCloudy (muted blue, drifting cloud shapes)

**Step 2: Revert preview symbol back to `"2"`**

Don't leave a non-default symbol in preview data.

**Step 3: Final commit**

```bash
git add ios/wby/wby/ContentView.swift
git commit -m "ios: add dynamic atmospheric background with SpriteKit particles"
```
