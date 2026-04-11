import Foundation

nonisolated struct ClimateNormalsResponse: Codable {
    let station: StationInfo
    let period: String
    let today: InterpolatedNormal
    let monthly: [MonthlyNormal]
}

struct InterpolatedNormal: Codable {
    let tempAvg: Double?
    let tempHigh: Double?
    let tempLow: Double?
    let precipMmDay: Double?
    let tempDiff: Double?

    enum CodingKeys: String, CodingKey {
        case tempAvg = "temp_avg"
        case tempHigh = "temp_high"
        case tempLow = "temp_low"
        case precipMmDay = "precip_mm_day"
        case tempDiff = "temp_diff"
    }
}

struct MonthlyNormal: Codable, Identifiable {
    let month: Int
    let tempAvg: Double?
    let tempHigh: Double?
    let tempLow: Double?
    let precipMm: Double?

    var id: Int { month }

    enum CodingKeys: String, CodingKey {
        case month
        case tempAvg = "temp_avg"
        case tempHigh = "temp_high"
        case tempLow = "temp_low"
        case precipMm = "precip_mm"
    }
}
