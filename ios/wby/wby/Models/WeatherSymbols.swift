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
        elevationMeters: Double
    ) -> WeatherScene {
        WeatherScene.from(symbolCode:
            nightAdjusted(primarySymbol(from: weather), coordinate: coordinate, at: date, elevationMeters: elevationMeters)
        )
    }

    static func nightAdjusted(
        _ symbolCode: String?,
        coordinate: CLLocationCoordinate2D,
        at date: Date,
        elevationMeters: Double
    ) -> String? {
        guard let code = symbolCode.flatMap(Int.init), code < 100 else { return symbolCode }
        let isNight = SunriseCard.isNight(coordinate: coordinate, date: date, elevationMeters: elevationMeters)
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
        case 2: return "Partly Cloudy"
        case 3: return "Mostly Cloudy"
        case 4, 5, 7: return "Overcast"
        case 6, 9: return "Fog"
        case 11: return "Showers"
        case 21: return "Light Showers"
        case 22: return "Showers"
        case 23: return "Heavy Showers"
        case 31: return "Light Rain"
        case 32: return "Rain"
        case 33: return "Heavy Rain"
        case 41: return "Light Snow"
        case 42: return "Snow"
        case 43: return "Heavy Snow"
        case 51: return "Light Sleet"
        case 52: return "Sleet"
        case 53: return "Heavy Sleet"
        case 61, 64: return "Thunderstorms"
        case 71, 74: return "Hail"
        default: return nil
        }
    }
}
