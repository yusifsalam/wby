import Foundation

struct LeaderboardResponse: Codable {
    let timeframe: String
    let leaderboard: [LeaderboardEntry]
}

struct LeaderboardEntry: Codable, Identifiable {
    let type: String
    let stationName: String
    let value: Double
    let unit: String
    let distanceKm: Double
    let observedAt: Date

    var id: String { type }

    enum CodingKeys: String, CodingKey {
        case type, value, unit
        case stationName = "station_name"
        case distanceKm = "distance_km"
        case observedAt = "observed_at"
    }
}
