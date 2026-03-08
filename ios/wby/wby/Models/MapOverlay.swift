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

    func expanded(by factor: Double) -> MapBBox {
        guard factor > 1 else { return clampedToWorld() }

        let lonSpan = maxLon - minLon
        let latSpan = maxLat - minLat
        let lonPad = lonSpan * (factor - 1) * 0.5
        let latPad = latSpan * (factor - 1) * 0.5

        return MapBBox(
            minLon: minLon - lonPad,
            minLat: minLat - latPad,
            maxLon: maxLon + lonPad,
            maxLat: maxLat + latPad
        ).clampedToWorld()
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

struct TemperatureOverlayImage: Equatable {
    let imageData: Data
    let bbox: MapBBox
    let dataTime: Date?
    let minTemp: Double?
    let maxTemp: Double?
}
