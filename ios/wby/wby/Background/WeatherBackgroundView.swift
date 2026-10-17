import SwiftUI

struct WeatherBackgroundView: View {
    let state: WeatherBackgroundState
    let sunFade: Double
    let isCovered: Bool

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.scenePhase) private var scenePhase
    @Environment(\.displayScale) private var displayScale
    @State private var clocks: MotionClocks
    @State private var wasBackgrounded = false
    @State private var strike: LightningStrike?

    init(state: WeatherBackgroundState, sunFade: Double = 1, isCovered: Bool = false) {
        self.state = state
        self.sunFade = sunFade
        self.isCovered = isCovered
        _clocks = State(initialValue: MotionClocks(rates: state.motionRates, at: .now))
    }

    var body: some View {
        GeometryReader { proxy in
            ZStack {
                TimelineView(.animation(minimumInterval: skyFrameInterval, paused: isPaused)) { context in
                    skyLayers(size: proxy.size, date: reduceMotion ? clocks.start : context.date)
                }
                TimelineView(.animation(minimumInterval: 1.0 / 60, paused: isPaused || !state.scene.hasPrecipitation)) { context in
                    precipitationLayers(size: proxy.size, date: reduceMotion ? clocks.start : context.date)
                }
            }
        }
        .allowsHitTesting(false)
        .onChange(of: state.motionRates) { _, rates in
            clocks.retarget(rates, at: .now)
        }
        .onChange(of: scenePhase) { _, phase in
            switch phase {
            case .background:
                wasBackgrounded = true
            case .active where wasBackgrounded:
                clocks = MotionClocks(rates: state.motionRates, at: .now)
                wasBackgrounded = false
            default:
                break
            }
        }
        .task(id: lightningActive) {
            guard lightningActive else {
                strike = nil
                return
            }
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(Double.random(in: 6...16)))
                if Task.isCancelled { break }
                strike = LightningStrike(start: .now, x: Float.random(in: 0.15...0.85))
                try? await Task.sleep(for: .seconds(LightningStrike.duration))
                strike = nil
            }
        }
    }

    private var isPaused: Bool {
        reduceMotion || isCovered || scenePhase != .active
    }

    private var lightningActive: Bool {
        state.scene == .storm && !isPaused
    }

    private var skyFrameInterval: Double {
        if strike != nil { return 1.0 / 60 }
        return sunFade > 0 ? 1.0 / 30 : 1.0 / 15
    }

    @ViewBuilder
    private func skyLayers(size: CGSize, date: Date) -> some View {
        let time = Float(date.timeIntervalSince(clocks.start))
        let style = state.scene.skyStyle(coverage: state.coverage)
        ZStack {
            if style.stars > 0 {
                StarsLayer(amount: style.stars, size: size, time: time)
                    .transition(.opacity)
            }

            SkyLayer(
                style: style,
                size: size,
                downscale: displayScale,
                time: time,
                drift: SIMD2<Float>(clocks.drift.value(at: date)),
                sunFade: Float(sunFade),
                flash: strike?.intensity(at: date) ?? 0,
                flashX: strike?.x ?? 0.5
            )
            .animation(.easeInOut(duration: 1.5), value: state)
        }
    }

    @ViewBuilder
    private func precipitationLayers(size: CGSize, date: Date) -> some View {
        let time = Float(date.timeIntervalSince(clocks.start))
        let flash = strike?.intensity(at: date) ?? 0
        ZStack {
            if state.scene.hasRain {
                RainLayer(
                    intensity: state.precipitationIntensity,
                    slant: state.rainSlant,
                    size: size,
                    phase: Float(clocks.rain.value(at: date).x)
                )
                .animation(.easeInOut(duration: 1.5), value: state)
                .transition(.opacity)
            }

            if state.scene == .snow {
                SnowLayer(
                    intensity: state.precipitationIntensity,
                    wind: Float(state.wind.x),
                    size: size,
                    time: time,
                    fall: SIMD2<Float>(clocks.snow.value(at: date))
                )
                .animation(.easeInOut(duration: 1.5), value: state)
                .transition(.opacity)
            }

            if flash > 0.001 {
                Color.white.opacity(Double(flash) * 0.12)
            }
        }
    }
}

private struct PhaseClock: Equatable {
    var anchorDate: Date
    var anchorValue: SIMD2<Double> = .zero
    var rate: SIMD2<Double>

    func value(at date: Date) -> SIMD2<Double> {
        anchorValue + rate * date.timeIntervalSince(anchorDate)
    }

    mutating func retarget(_ newRate: SIMD2<Double>, at date: Date) {
        anchorValue = value(at: date)
        anchorDate = date
        rate = newRate
    }
}

private struct MotionClocks: Equatable {
    var start: Date
    var drift: PhaseClock
    var rain: PhaseClock
    var snow: PhaseClock

    init(rates: MotionRates, at date: Date) {
        start = date
        drift = PhaseClock(anchorDate: date, rate: rates.drift)
        rain = PhaseClock(anchorDate: date, rate: SIMD2(rates.rain, 0))
        snow = PhaseClock(anchorDate: date, rate: rates.snow)
    }

    mutating func retarget(_ rates: MotionRates, at date: Date) {
        drift.retarget(rates.drift, at: date)
        rain.retarget(SIMD2(rates.rain, 0), at: date)
        snow.retarget(rates.snow, at: date)
    }
}

private struct LightningStrike: Equatable {
    var start: Date
    var x: Float

    static let duration = 1.2

    func intensity(at date: Date) -> Float {
        let t = date.timeIntervalSince(start)
        guard t >= 0, t < Self.duration else { return 0 }
        return Float(max(
            pulse(t, at: 0, peak: 1, decay: 0.06),
            pulse(t, at: 0.12, peak: 0.55, decay: 0.05),
            pulse(t, at: 0.27, peak: 0.85, decay: 0.14)
        ))
    }

    private func pulse(_ t: Double, at onset: Double, peak: Double, decay: Double) -> Double {
        let x = t - onset
        guard x >= 0 else { return 0 }
        return peak * min(x / 0.015, 1) * exp(-x / decay)
    }
}

private struct WeatherBackgroundPreview: View {
    let state: WeatherBackgroundState

    var body: some View {
        WeatherBackgroundView(state: state)
            .background(
                LinearGradient(
                    colors: state.scene.gradientColors,
                    startPoint: .top,
                    endPoint: .bottom
                )
            )
            .ignoresSafeArea()
    }
}

#Preview("Clear day") {
    WeatherBackgroundPreview(state: WeatherBackgroundState(scene: .clearDay, cloudCover: 1))
}

#Preview("Clear night") {
    WeatherBackgroundPreview(state: WeatherBackgroundState(scene: .clearNight))
}

#Preview("Partly cloudy") {
    WeatherBackgroundPreview(state: WeatherBackgroundState(scene: .partlyCloudy, cloudCover: 5, windSpeed: 6, windDirection: 250))
}

#Preview("Partly cloudy night") {
    WeatherBackgroundPreview(state: WeatherBackgroundState(scene: .partlyCloudyNight, cloudCover: 5))
}

#Preview("Overcast") {
    WeatherBackgroundPreview(state: WeatherBackgroundState(scene: .overcast, cloudCover: 8, windSpeed: 4, windDirection: 180))
}

#Preview("Rain") {
    WeatherBackgroundPreview(state: WeatherBackgroundState(scene: .rain, precipitation1h: 0.6, cloudCover: 8, windSpeed: 7, windDirection: 240))
}

#Preview("Snow") {
    WeatherBackgroundPreview(state: WeatherBackgroundState(scene: .snow, precipitation1h: 0.8, cloudCover: 8, windSpeed: 4, windDirection: 300))
}

#Preview("Storm") {
    WeatherBackgroundPreview(state: WeatherBackgroundState(scene: .storm, precipitation1h: 3.0, cloudCover: 8, windSpeed: 12, windDirection: 220))
}
