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
        case 2, 3, 4, 6:
            return isNight ? .partlyCloudyNight : .partlyCloudy
        case 5, 7, 9:
            return .overcast
        case 14, 17, 41, 42, 43, 44, 45, 46, 47, 48, 49:
            return .overcast
        case 11, 21, 24, 27, 31, 32, 33, 34, 35, 36, 37, 38, 39:
            return .rain
        case 51, 52, 53, 54, 55, 56, 57, 58, 59:
            return .snow
        case 61, 64, 67:
            return .snow
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

    var prefersLightForeground: Bool {
        switch self {
        case .clearNight, .partlyCloudyNight, .overcast, .rain, .storm:
            return true
        case .clearDay, .partlyCloudy, .snow:
            return false
        }
    }

    var hasRain: Bool { self == .rain || self == .storm }

    var hasPrecipitation: Bool { hasRain || self == .snow }

    func skyStyle(coverage: Float) -> SkyStyle {
        switch self {
        case .clearDay:
            return SkyStyle(
                cumulus: coverage * 0.12, cirrus: 0.30, absorption: 0.30, sun: 1,
                lit: SkyStyle.rgb(1.0, 0.99, 0.97), shadow: SkyStyle.rgb(0.70, 0.79, 0.92)
            )
        case .clearNight:
            return SkyStyle(
                cumulus: coverage * 0.12, cirrus: 0.15, absorption: 0.40, stars: 1, night: 1,
                lit: SkyStyle.rgb(0.30, 0.33, 0.42), shadow: SkyStyle.rgb(0.10, 0.12, 0.21)
            )
        case .partlyCloudy:
            return SkyStyle(
                cumulus: 0.25 + coverage * 0.55, cirrus: 0.40, absorption: 0.45, sun: 1,
                lit: SkyStyle.rgb(1.0, 0.99, 0.96), shadow: SkyStyle.rgb(0.60, 0.68, 0.80)
            )
        case .partlyCloudyNight:
            return SkyStyle(
                cumulus: 0.25 + coverage * 0.55, cirrus: 0.20, absorption: 0.50, stars: 1, night: 1,
                lit: SkyStyle.rgb(0.34, 0.37, 0.46), shadow: SkyStyle.rgb(0.12, 0.15, 0.24)
            )
        case .overcast:
            return SkyStyle(
                cumulus: 1, deck: 1, absorption: 0.60,
                lit: SkyStyle.rgb(0.52, 0.57, 0.64), shadow: SkyStyle.rgb(0.30, 0.35, 0.43)
            )
        case .rain:
            return SkyStyle(
                cumulus: 1, deck: 1, absorption: 0.90,
                lit: SkyStyle.rgb(0.42, 0.47, 0.54), shadow: SkyStyle.rgb(0.18, 0.22, 0.29)
            )
        case .snow:
            return SkyStyle(
                cumulus: 1, deck: 1, absorption: 0.35,
                lit: SkyStyle.rgb(0.90, 0.92, 0.95), shadow: SkyStyle.rgb(0.64, 0.70, 0.78)
            )
        case .storm:
            return SkyStyle(
                cumulus: 1, deck: 1, absorption: 1.30,
                lit: SkyStyle.rgb(0.28, 0.30, 0.38), shadow: SkyStyle.rgb(0.07, 0.08, 0.12)
            )
        }
    }
}

struct SkyStyle {
    var cumulus: Float
    var deck: Float = 0
    var cirrus: Float = 0
    var absorption: Float
    var sun: Float = 0
    var stars: Float = 0
    var night: Float = 0
    var lit: Color.Resolved
    var shadow: Color.Resolved

    static func rgb(_ red: Float, _ green: Float, _ blue: Float) -> Color.Resolved {
        Color.Resolved(red: red, green: green, blue: blue)
    }
}
