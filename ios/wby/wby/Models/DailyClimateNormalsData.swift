import Foundation

nonisolated struct DailyClimateNormalsResponse: Codable {
    let station: StationInfo
    let period: String
    let today: DailyNormalToday
    let daily: [DailyNormal]
}

struct DailyNormal: Codable, Identifiable {
    let month: Int
    let day: Int
    let tempAvg: Double?
    let tempHigh: Double?
    let tempLow: Double?
    let precipMm: Double?

    var id: Int { month * 100 + day }

    enum CodingKeys: String, CodingKey {
        case month
        case day
        case tempAvg = "temp_avg"
        case tempHigh = "temp_high"
        case tempLow = "temp_low"
        case precipMm = "precip_mm"
    }
}

struct DailyNormalToday: Codable {
    let month: Int
    let day: Int
    let tempAvg: Double?
    let tempHigh: Double?
    let tempLow: Double?
    let precipMm: Double?
    let tempHourly: [Double]?
    let tempNowNormal: Double?
    let tempDiff: Double?

    enum CodingKeys: String, CodingKey {
        case month
        case day
        case tempAvg = "temp_avg"
        case tempHigh = "temp_high"
        case tempLow = "temp_low"
        case precipMm = "precip_mm"
        case tempHourly = "temp_hourly"
        case tempNowNormal = "temp_now_normal"
        case tempDiff = "temp_diff"
    }
}
