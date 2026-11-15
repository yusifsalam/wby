import Foundation

/// Compass names for FMI wind directions, which give where the wind blows
/// from (0° = north, 90° = east).
enum WindDirection {
    private static let points = [
        ("N", "north"), ("NNE", "north-northeast"), ("NE", "northeast"), ("ENE", "east-northeast"),
        ("E", "east"), ("ESE", "east-southeast"), ("SE", "southeast"), ("SSE", "south-southeast"),
        ("S", "south"), ("SSW", "south-southwest"), ("SW", "southwest"), ("WSW", "west-southwest"),
        ("W", "west"), ("WNW", "west-northwest"), ("NW", "northwest"), ("NNW", "north-northwest"),
    ]

    /// Eight-point abbreviation, e.g. "SW".
    static func abbreviation(_ degrees: Double) -> String {
        let index = Int((normalized(degrees) + 22.5) / 45) % 8
        return points[index * 2].0
    }

    /// Sixteen-point name, e.g. "south-southwest".
    static func name(_ degrees: Double) -> String {
        let index = Int((normalized(degrees) + 11.25) / 22.5) % 16
        return points[index].1
    }

    private static func normalized(_ degrees: Double) -> Double {
        let value = degrees.truncatingRemainder(dividingBy: 360)
        return value < 0 ? value + 360 : value
    }
}
