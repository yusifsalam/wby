import Foundation

nonisolated struct DailyClimateNormalsResponse: Codable {
    let station: StationInfo
    let period: String
    let today: DailyNormalToday
    let precipitation: DailyNormalsPrecipitation?
    let daily: [DailyNormal]
}

struct DailyNormalsPrecipitation: Codable {
    let station: StationInfo?
    let todayObservedMm: Double?
    let todayNormalMm: Double?
    let monthToDateObservedMm: Double?
    let monthToDateNormalMm: Double?
    let monthNormalMm: Double?
    let observedThrough: Date?

    enum CodingKeys: String, CodingKey {
        case station
        case todayObservedMm = "today_observed_mm"
        case todayNormalMm = "today_normal_mm"
        case monthToDateObservedMm = "month_to_date_observed_mm"
        case monthToDateNormalMm = "month_to_date_normal_mm"
        case monthNormalMm = "month_normal_mm"
        case observedThrough = "observed_through"
    }
}

struct DailyNormal: Codable, Identifiable {
    let month: Int
    let day: Int
    let tempAvg: Double?
    let tempHigh: Double?
    let tempLow: Double?
    let feelsLikeAvg: Double?
    let feelsLikeHigh: Double?
    let feelsLikeLow: Double?
    let windAvg: Double?
    let windGust: Double?
    let humidityAvg: Double?
    let precipMm: Double?
    let precipDaysPct: Double?
    let snowCm: Double?

    var id: Int { month * 100 + day }

    enum CodingKeys: String, CodingKey {
        case month
        case day
        case tempAvg = "temp_avg"
        case tempHigh = "temp_high"
        case tempLow = "temp_low"
        case feelsLikeAvg = "feels_like_avg"
        case feelsLikeHigh = "feels_like_high"
        case feelsLikeLow = "feels_like_low"
        case windAvg = "wind_avg"
        case windGust = "wind_gust"
        case humidityAvg = "humidity_avg"
        case precipMm = "precip_mm"
        case precipDaysPct = "precip_days_pct"
        case snowCm = "snow_cm"
    }
}

struct DailyNormalToday: Codable {
    let month: Int
    let day: Int
    let tempAvg: Double?
    let tempHigh: Double?
    let tempLow: Double?
    let feelsLikeAvg: Double?
    let feelsLikeHigh: Double?
    let feelsLikeLow: Double?
    let windAvg: Double?
    let windGust: Double?
    let humidityAvg: Double?
    let precipMm: Double?
    let precipDaysPct: Double?
    let snowCm: Double?
    let tempHourly: [Double]?
    let tempHourlyP10: [Double]?
    let tempHourlyP90: [Double]?
    let feelsLikeHourly: [Double]?
    let windHourly: [Double]?
    let humidityHourly: [Double]?
    let tempNowNormal: Double?
    let tempDiff: Double?
    let feelsLikeNowNormal: Double?
    let windNowNormal: Double?
    let humidityNowNormal: Double?

    enum CodingKeys: String, CodingKey {
        case month
        case day
        case tempAvg = "temp_avg"
        case tempHigh = "temp_high"
        case tempLow = "temp_low"
        case feelsLikeAvg = "feels_like_avg"
        case feelsLikeHigh = "feels_like_high"
        case feelsLikeLow = "feels_like_low"
        case windAvg = "wind_avg"
        case windGust = "wind_gust"
        case humidityAvg = "humidity_avg"
        case precipMm = "precip_mm"
        case precipDaysPct = "precip_days_pct"
        case snowCm = "snow_cm"
        case tempHourly = "temp_hourly"
        case tempHourlyP10 = "temp_hourly_p10"
        case tempHourlyP90 = "temp_hourly_p90"
        case feelsLikeHourly = "feels_like_hourly"
        case windHourly = "wind_hourly"
        case humidityHourly = "humidity_hourly"
        case tempNowNormal = "temp_now_normal"
        case tempDiff = "temp_diff"
        case feelsLikeNowNormal = "feels_like_now_normal"
        case windNowNormal = "wind_now_normal"
        case humidityNowNormal = "humidity_now_normal"
    }
}
