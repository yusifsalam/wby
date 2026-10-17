import Foundation

struct WeatherBackgroundState: Equatable {
    var scene: WeatherScene
    var precipitation1h: Double?
    var cloudCover: Double?
    var windSpeed: Double?
    var windDirection: Double?

    static let placeholder = WeatherBackgroundState(scene: .clearDay)

    init(
        scene: WeatherScene,
        precipitation1h: Double? = nil,
        cloudCover: Double? = nil,
        windSpeed: Double? = nil,
        windDirection: Double? = nil
    ) {
        self.scene = scene
        self.precipitation1h = precipitation1h
        self.cloudCover = cloudCover
        self.windSpeed = windSpeed
        self.windDirection = windDirection
    }

    /// Cloud cover in 0...1. FMI may report oktas (0-8) or percent (0-100);
    /// matches the display logic in CurrentConditionsCard.formatCloudCover.
    var coverage: Float {
        guard let cover = cloudCover else { return 0.5 }
        let normalized: Double = cover <= 8.5 ? cover / 8.0 : cover / 100.0
        return Float(min(max(normalized, 0.0), 1.0))
    }

    /// Precipitation intensity in 0...3, drizzle through torrential.
    var precipitationIntensity: Float {
        let isStorm = scene == .storm
        let base: Float = isStorm ? 2.4 : 0.9
        let floor: Float = isStorm ? 2.0 : 0.15
        guard let mm = precipitation1h, mm > 0 else { return base }
        return min(max(Float(sqrt(mm / 2.0)), floor), 3.0)
    }

    /// The direction the wind blows toward in m/s: x east, y north.
    var wind: SIMD2<Double> {
        let speed = windSpeed ?? 3
        let radians = (windDirection ?? 270) * .pi / 180
        return SIMD2(-sin(radians), -cos(radians)) * speed
    }

    var rainSlant: Float {
        Float(min(max(wind.x * 0.03, -0.35), 0.35))
    }

    var motionRates: MotionRates {
        let speed = (wind * wind).sum().squareRoot()
        let direction = speed > 0.01 ? wind / speed : SIMD2(1, 0)
        let w = Double(min(max(precipitationIntensity / 1.8, 0), 1))
        return MotionRates(
            drift: direction * min(max(speed, 1.5), 15) * 0.0072,
            rain: 280 + (740 - 280) * w,
            snow: SIMD2(min(max(wind.x * 8, -70), 70), 120)
        )
    }
}

struct MotionRates: Equatable {
    var drift: SIMD2<Double>
    var rain: Double
    var snow: SIMD2<Double>
}
