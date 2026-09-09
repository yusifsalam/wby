import Foundation
import SwiftUI

/// WHO UV index bands and the per-day slicing of the hourly UV forecast.
enum UVIndex {
    static let scaleMax = 11.0

    struct Band {
        let label: String
        let color: Color
        let limit: Double
    }

    static let bands: [Band] = [
        Band(label: "Low", color: Color(red: 0.44, green: 0.84, blue: 0.36), limit: 3),
        Band(label: "Moderate", color: Color(red: 0.90, green: 0.88, blue: 0.25), limit: 6),
        Band(label: "High", color: Color(red: 0.95, green: 0.58, blue: 0.27), limit: 8),
        Band(label: "Very high", color: Color(red: 0.95, green: 0.32, blue: 0.34), limit: 11),
        Band(label: "Extreme", color: Color(red: 0.80, green: 0.30, blue: 0.90), limit: .infinity),
    ]

    static func band(for value: Double) -> Band {
        bands.first { value < $0.limit } ?? bands[bands.count - 1]
    }

    static func level(for value: Double) -> String {
        band(for: value).label
    }

    static func color(for value: Double) -> Color {
        band(for: value).color
    }

    /// Bar height as a 0–1 fraction of the scale.
    static func barFraction(_ value: Double) -> Double {
        min(max(value, 0), scaleMax) / scaleMax
    }

    struct Peak {
        let value: Double
        let time: Date
    }

    /// Highest UV index among the points and the first hour it is reached.
    static func peak(_ points: [UVPoint]) -> Peak? {
        var peak: Peak?
        for point in points where point.uv.isFinite {
            if let current = peak, point.uv <= current.value { continue }
            peak = Peak(value: point.uv, time: point.time)
        }
        return peak
    }

    /// Points falling on the local calendar day `offsetDays` after `now`.
    static func dayPoints(_ points: [UVPoint], timeZone: TimeZone, now: Date = Date(), offsetDays: Int = 0) -> [UVPoint] {
        var calendar = Calendar.current
        calendar.timeZone = timeZone
        guard let target = calendar.date(byAdding: .day, value: offsetDays, to: now) else { return [] }
        return points.filter { calendar.isDate($0.time, inSameDayAs: target) }
    }

    struct DayBar: Identifiable {
        let hour: Int
        var time: Date?
        var uv: Double?
        let past: Bool
        let current: Bool

        var id: Int { hour }
    }

    /// The 24 hourly slots of today in the local time zone. Hours the forecast
    /// does not cover have a nil value.
    static func dayBars(_ points: [UVPoint], timeZone: TimeZone, now: Date = Date()) -> [DayBar] {
        var calendar = Calendar.current
        calendar.timeZone = timeZone
        let currentHour = calendar.component(.hour, from: now)
        var bars = (0..<24).map { hour in
            DayBar(hour: hour, past: hour < currentHour, current: hour == currentHour)
        }
        for point in dayPoints(points, timeZone: timeZone, now: now) where point.uv.isFinite {
            let hour = calendar.component(.hour, from: point.time)
            bars[hour].time = point.time
            bars[hour].uv = point.uv
        }
        return bars
    }
}
