import CoreLocation
import Foundation

enum WeatherSymbols {
    static func primarySymbol(from weather: WeatherResponse?) -> String? {
        weather?.hourlyForecast.first?.symbol ?? weather?.dailyForecast.first?.symbol
    }

    static func scene(for weather: WeatherResponse?) -> WeatherScene {
        WeatherScene.from(symbolCode: primarySymbol(from: weather))
    }

    static func scene(
        for weather: WeatherResponse?,
        coordinate: CLLocationCoordinate2D,
        date: Date,
        timeZone: TimeZone,
        elevationMeters: Double
    ) -> WeatherScene {
        WeatherScene.from(symbolCode:
            nightAdjusted(
                primarySymbol(from: weather),
                coordinate: coordinate,
                at: date,
                timeZone: timeZone,
                elevationMeters: elevationMeters
            )
        )
    }

    static func nightAdjusted(
        _ symbolCode: String?,
        coordinate: CLLocationCoordinate2D,
        at date: Date,
        timeZone: TimeZone,
        elevationMeters: Double
    ) -> String? {
        guard let code = symbolCode.flatMap(Int.init), code < 100 else { return symbolCode }
        let isNight = SunriseCard.isNight(
            coordinate: coordinate,
            date: date,
            timeZone: timeZone,
            elevationMeters: elevationMeters
        )
        return isNight ? String(code + 100) : symbolCode
    }

    static func conditionDescription(from weather: WeatherResponse?) -> String? {
        conditionDescription(from: primarySymbol(from: weather))
    }

    static func conditionDescription(from symbolCode: String?) -> String? {
        guard let code = symbolCode.flatMap(Int.init) else { return nil }
        let n = code >= 100 ? code - 100 : code
        switch n {
        case 1: return "Clear"
        case 2: return "Mostly Clear"
        case 4: return "Partly Cloudy"
        case 6: return "Mostly Cloudy"
        case 7: return "Cloudy"
        case 9: return "Fog"
        case 11: return "Drizzle"
        case 14: return "Freezing Drizzle"
        case 17: return "Freezing Rain"
        case 21: return "Isolated Showers"
        case 24: return "Scattered Showers"
        case 27: return "Showers"
        case 31: return "Isolated Light Showers"
        case 32: return "Isolated Showers"
        case 33: return "Isolated Heavy Showers"
        case 34: return "Scattered Light Showers"
        case 35: return "Scattered Showers"
        case 36: return "Scattered Heavy Showers"
        case 37: return "Light Rain"
        case 38: return "Rain"
        case 39: return "Heavy Rain"
        case 41: return "Isolated Light Sleet Showers"
        case 42: return "Isolated Sleet Showers"
        case 43: return "Isolated Heavy Sleet Showers"
        case 44: return "Scattered Light Sleet Showers"
        case 45: return "Scattered Sleet Showers"
        case 46: return "Scattered Heavy Sleet Showers"
        case 47: return "Light Sleet"
        case 48: return "Sleet"
        case 49: return "Heavy Sleet"
        case 51: return "Isolated Light Snow Showers"
        case 52: return "Isolated Snow Showers"
        case 53: return "Isolated Heavy Snow Showers"
        case 54: return "Scattered Light Snow Showers"
        case 55: return "Scattered Snow Showers"
        case 56: return "Scattered Heavy Snow Showers"
        case 57: return "Light Snow"
        case 58: return "Snow"
        case 59: return "Heavy Snow"
        case 61: return "Isolated Hail Showers"
        case 64: return "Scattered Hail Showers"
        case 67: return "Hail Showers"
        case 71: return "Isolated Thundershowers"
        case 74: return "Scattered Thundershowers"
        case 77: return "Thundershowers"
        default: return nil
        }
    }
}
