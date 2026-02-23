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
}
