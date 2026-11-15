import SwiftUI

enum TemperatureColor {
    static func color(for temp: Double) -> Color {
        switch temp {
        case ..<(-30): return Color(red: 0.23, green: 0.08, blue: 0.36)
        case ..<(-20): return Color(red: 0.27, green: 0.19, blue: 0.59)
        case ..<(-10): return Color(red: 0.20, green: 0.36, blue: 0.83)
        case ..<0: return Color(red: 0.20, green: 0.58, blue: 0.95)
        case ..<10: return Color(red: 0.22, green: 0.77, blue: 0.72)
        case ..<20: return Color(red: 0.94, green: 0.82, blue: 0.28)
        case ..<30: return Color(red: 0.93, green: 0.49, blue: 0.23)
        default: return Color(red: 0.57, green: 0.10, blue: 0.12)
        }
    }

    /// A vertical gradient for a plot spanning `min`...`max`, blending the
    /// bands through their centers so a line shades smoothly with height.
    static func verticalGradient(min: Double, max: Double) -> LinearGradient {
        let range = Swift.max(max - min, 0.001)
        let centers = stride(from: 35.0, through: -35.0, by: -10.0).filter { $0 < max && $0 > min }
        let values = [max] + centers + [min]
        let stops = values.map { Gradient.Stop(color: color(for: $0), location: (max - $0) / range) }
        return LinearGradient(stops: stops, startPoint: .top, endPoint: .bottom)
    }
}
