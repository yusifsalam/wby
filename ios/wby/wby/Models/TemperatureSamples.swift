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
    /// Set when the field came from the dense GRIB forecast raster; the renderer
    /// prefers it (texture bilinear) over `samples`. Nil for station fallbacks.
    let grid: TemperatureGrid?

    enum CodingKeys: String, CodingKey {
        case dataTime = "data_time"
        case minTemp = "min_temp"
        case maxTemp = "max_temp"
        case samples
        case grid
    }
}

/// Harmonie precipitation-rate forecast raster (mm/h) at one hour, rendered
/// client-side via texture bilinear. Reuses `TemperatureGrid` for the raster.
nonisolated struct PrecipitationForecastResponse: Codable, Equatable {
    let dataTime: Date
    let min: Double
    let max: Double
    let grid: TemperatureGrid?

    enum CodingKeys: String, CodingKey {
        case dataTime = "data_time"
        case min
        case max
        case grid
    }
}

/// A regular lat/lon raster of field values (°C for temperature, mm/h for
/// precipitation). `values` is row-major, north-to-south (row 0 = `maxLat`),
/// west-to-east, length `rows*cols`; a nil entry is a masked cell.
nonisolated struct TemperatureGrid: Codable, Equatable {
    let rows: Int
    let cols: Int
    let minLat: Double
    let maxLat: Double
    let minLon: Double
    let maxLon: Double
    let values: [Double?]

    enum CodingKeys: String, CodingKey {
        case rows
        case cols
        case minLat = "min_lat"
        case maxLat = "max_lat"
        case minLon = "min_lon"
        case maxLon = "max_lon"
        case values
    }
}
