import Foundation

struct MapBBox: Equatable, Hashable {
    let minLon: Double
    let minLat: Double
    let maxLon: Double
    let maxLat: Double

    var queryValue: String {
        [
            Self.coordinateString(minLon),
            Self.coordinateString(minLat),
            Self.coordinateString(maxLon),
            Self.coordinateString(maxLat),
        ].joined(separator: ",")
    }

    private static func coordinateString(_ value: Double) -> String {
        String(format: "%.5f", locale: Locale(identifier: "en_US_POSIX"), value)
    }
}

struct TemperatureOverlayImage: Equatable {
    let imageData: Data
    let bbox: MapBBox
    let dataTime: Date?
    let minTemp: Double?
    let maxTemp: Double?
}
