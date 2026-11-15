import Foundation

nonisolated struct MapBBox: Equatable, Hashable {
    /// Full Finland bounding box matching the FMI observation coverage.
    static let finland = MapBBox(minLon: 19.0, minLat: 59.0, maxLon: 32.0, maxLat: 71.5)

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

    func clampedToWorld() -> MapBBox {
        MapBBox(
            minLon: Self.clamp(minLon, min: -180, max: 180),
            minLat: Self.clamp(minLat, min: -90, max: 90),
            maxLon: Self.clamp(maxLon, min: -180, max: 180),
            maxLat: Self.clamp(maxLat, min: -90, max: 90)
        )
    }

    private static func coordinateString(_ value: Double) -> String {
        String(format: "%.5f", locale: Locale(identifier: "en_US_POSIX"), value)
    }

    private static func clamp(_ value: Double, min: Double, max: Double) -> Double {
        if value < min { return min }
        if value > max { return max }
        return value
    }
}
