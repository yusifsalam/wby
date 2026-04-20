import SwiftUI

struct WeatherBackgroundView: View {
    let weatherScene: WeatherScene
    let precipitation1h: Double?
    let cloudCover: Double?
    let sunOpacity: Double

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.scenePhase) private var scenePhase
    @State private var startDate = Date()

    init(
        weatherScene: WeatherScene,
        precipitation1h: Double?,
        cloudCover: Double?,
        sunOpacity: Double = 1.0
    ) {
        self.weatherScene = weatherScene
        self.precipitation1h = precipitation1h
        self.cloudCover = cloudCover
        self.sunOpacity = sunOpacity
    }

    var body: some View {
        GeometryReader { proxy in
            let size = proxy.size
            TimelineView(.animation(paused: isPaused)) { context in
                let elapsed = reduceMotion
                    ? 0
                    : Float(context.date.timeIntervalSince(startDate))
                content(size: size, time: elapsed)
            }
        }
        .allowsHitTesting(false)
    }

    private var isPaused: Bool {
        reduceMotion || scenePhase != .active
    }

    @ViewBuilder
    private func content(size: CGSize, time: Float) -> some View {
        ZStack {
            if showsSun {
                SunLayer(time: time, size: size)
                    .opacity(sunOpacity)
                    .transition(.opacity)
            }
            if showsStars {
                StarsLayer(time: time, size: size)
                    .transition(.opacity)
            }
            if showsClouds {
                CloudsLayer(time: time, size: size, coverage: coverage, tint: cloudTint)
                    .transition(.opacity)
            }
            if showsRain {
                RainLayer(time: time, size: size, intensity: precipIntensity(stormBase: weatherScene == .storm))
                    .transition(.opacity)
            }
            if showsSnow {
                SnowLayer(time: time, size: size, intensity: precipIntensity(stormBase: false))
                    .transition(.opacity)
            }
            if showsLightning {
                LightningLayer(active: !reduceMotion)
                    .transition(.opacity)
            }
        }
        .animation(.easeInOut(duration: 0.6), value: weatherScene)
    }

    // MARK: - Scene routing

    private var showsSun: Bool {
        weatherScene == .clearDay
    }

    private var showsStars: Bool {
        weatherScene == .clearNight || weatherScene == .partlyCloudyNight
    }

    private var showsClouds: Bool {
        weatherScene == .partlyCloudy || weatherScene == .partlyCloudyNight
    }

    private var showsRain: Bool {
        weatherScene == .rain || weatherScene == .storm
    }

    private var showsSnow: Bool { weatherScene == .snow }

    private var showsLightning: Bool { weatherScene == .storm }

    private var cloudTint: SIMD4<Float> {
        switch weatherScene {
        case .partlyCloudyNight:
            return SIMD4(0.55, 0.60, 0.70, 0.55)
        default:
            return SIMD4(1.0, 1.0, 1.0, 0.85)
        }
    }

    private func precipIntensity(stormBase: Bool) -> Float {
        let base: Float = stormBase ? 2.4 : 0.9
        let floor: Float = stormBase ? 2.0 : 0.15
        guard let mm = precipitation1h, mm > 0 else { return base }
        let v = Float(sqrt(mm / 2.0))
        return min(max(v, floor), 3.0)
    }

    // Normalizes cloudCover, which FMI may report as oktas (0-8) or percent (0-100).
    // Matches the display logic in CurrentConditionsCard.formatCloudCover.
    private var coverage: Float {
        guard let cover = cloudCover else { return 0.5 }
        let normalized: Double = cover <= 8.5 ? cover / 8.0 : cover / 100.0
        return Float(min(max(normalized, 0.0), 1.0))
    }
}

#Preview("Clear night") {
    WeatherBackgroundView(weatherScene: .clearNight, precipitation1h: nil, cloudCover: nil)
        .background(
            LinearGradient(
                colors: WeatherScene.clearNight.gradientColors,
                startPoint: .top,
                endPoint: .bottom
            )
        )
        .ignoresSafeArea()
}

#Preview("Partly cloudy") {
    WeatherBackgroundView(weatherScene: .partlyCloudy, precipitation1h: nil, cloudCover: 6)
        .background(
            LinearGradient(
                colors: WeatherScene.partlyCloudy.gradientColors,
                startPoint: .top,
                endPoint: .bottom
            )
        )
        .ignoresSafeArea()
}

#Preview("Rain") {
    WeatherBackgroundView(weatherScene: .rain, precipitation1h: 0.25, cloudCover: 7)
        .background(
            LinearGradient(
                colors: WeatherScene.rain.gradientColors,
                startPoint: .top,
                endPoint: .bottom
            )
        )
        .ignoresSafeArea()
}

#Preview("Snow") {
    WeatherBackgroundView(weatherScene: .snow, precipitation1h: 0.8, cloudCover: 8)
        .background(
            LinearGradient(
                colors: WeatherScene.snow.gradientColors,
                startPoint: .top,
                endPoint: .bottom
            )
        )
        .ignoresSafeArea()
}

#Preview("Storm") {
    WeatherBackgroundView(weatherScene: .storm, precipitation1h: 3.0, cloudCover: 8)
        .background(
            LinearGradient(
                colors: WeatherScene.storm.gradientColors,
                startPoint: .top,
                endPoint: .bottom
            )
        )
        .ignoresSafeArea()
}
