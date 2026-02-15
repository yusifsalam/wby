import Foundation

struct WeatherResponse: Codable {
    let station: StationInfo
    let current: CurrentConditions
    let hourlyForecast: [HourlyForecast]
    let dailyForecast: [DailyForecast]

    enum CodingKeys: String, CodingKey {
        case station
        case current
        case hourlyForecast = "hourly_forecast"
        case dailyForecast = "daily_forecast"
    }

    init(station: StationInfo, current: CurrentConditions, hourlyForecast: [HourlyForecast], dailyForecast: [DailyForecast]) {
        self.station = station
        self.current = current
        self.hourlyForecast = hourlyForecast
        self.dailyForecast = dailyForecast
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        station = try c.decode(StationInfo.self, forKey: .station)
        current = try c.decode(CurrentConditions.self, forKey: .current)
        hourlyForecast = try c.decodeIfPresent([HourlyForecast].self, forKey: .hourlyForecast) ?? []
        dailyForecast = try c.decode([DailyForecast].self, forKey: .dailyForecast)
    }
}

struct StationInfo: Codable {
    let name: String
    let distanceKm: Double

    enum CodingKeys: String, CodingKey {
        case name
        case distanceKm = "distance_km"
    }
}

struct CurrentConditions: Codable {
    let temperature: Double?
    let feelsLike: Double?
    let windSpeed: Double?
    let windGust: Double?
    let windDirection: Double?
    let humidity: Double?
    let pressure: Double?
    let observedAt: Date

    enum CodingKeys: String, CodingKey {
        case temperature
        case feelsLike = "feels_like"
        case windSpeed = "wind_speed"
        case windGust = "wind_gust"
        case windDirection = "wind_direction"
        case humidity
        case pressure
        case observedAt = "observed_at"
    }
}

struct DailyForecast: Codable, Identifiable {
    let date: String
    let high: Double?
    let low: Double?
    let symbol: String?
    let windSpeedAvg: Double?
    let precipitationMm: Double?

    var id: String {
        date
    }

    var displayDate: Date? {
        let formatter = DateFormatter()
        formatter.dateFormat = "yyyy-MM-dd"
        return formatter.date(from: date)
    }

    enum CodingKeys: String, CodingKey {
        case date, high, low, symbol
        case windSpeedAvg = "wind_speed_avg"
        case precipitationMm = "precipitation_mm"
    }
}

struct HourlyForecast: Codable, Identifiable {
    let time: Date
    let temperature: Double?
    let symbol: String?

    var id: Date { time }
}
