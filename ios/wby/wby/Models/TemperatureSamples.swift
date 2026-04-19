import Foundation

nonisolated struct TemperatureSample: Codable, Equatable {
    let lat: Double
    let lon: Double
    let temp: Double
    let observedAt: Date

    enum CodingKeys: String, CodingKey {
        case lat
        case lon
        case temp
        case observedAt = "observed_at"
    }
}

nonisolated struct TemperatureSamplesResponse: Codable, Equatable {
    let dataTime: Date
    let minTemp: Double
    let maxTemp: Double
    let samples: [TemperatureSample]

    enum CodingKeys: String, CodingKey {
        case dataTime = "data_time"
        case minTemp = "min_temp"
        case maxTemp = "max_temp"
        case samples
    }
}
