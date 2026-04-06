import Foundation

struct LeaderboardResponse: Codable {
    let timeframe: String
    let leaderboard: [LeaderboardEntry]
}

struct LeaderboardEntry: Codable, Identifiable {
    let type: String
    let stationName: String
    let lat: Double?
    let lon: Double?
    let value: Double
    let unit: String
    let distanceKm: Double
    let observedAt: Date

    var id: String { type }

    enum CodingKeys: String, CodingKey {
        case type, lat, lon, value, unit
        case stationName = "station_name"
        case distanceKm = "distance_km"
        case observedAt = "observed_at"
    }
}
