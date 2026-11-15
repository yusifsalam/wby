import CoreGraphics
import Foundation

/// A linear value axis with evenly spaced ticks for the hand-drawn charts.
struct ChartScale {
    let min: Double
    let max: Double
    let ticks: [Double]

    func y(_ value: Double, height: CGFloat) -> CGFloat {
        let range = Swift.max(max - min, 0.001)
        return height * (1 - CGFloat((value - min) / range))
    }

    /// Covers `low`...`high` with a margin, snapped outward to whole steps.
    static func covering(_ low: Double, _ high: Double, divisions: Double) -> ChartScale {
        let lower = low - 1
        let upper = high + 1
        let step = niceStep((upper - lower) / divisions)
        let min = floor(lower / step) * step
        let max = ceil(upper / step) * step
        return ChartScale(min: min, max: max, ticks: ticks(from: min, to: max, step: step))
    }

    /// Starts at zero and reaches at least `high`, snapped up to a whole step.
    static func fromZero(to high: Double, divisions: Double) -> ChartScale {
        let step = niceStep(Swift.max(high, 1) / divisions)
        let max = Swift.max(ceil(high / step), 1) * step
        return ChartScale(min: 0, max: max, ticks: ticks(from: 0, to: max, step: step))
    }

    static func ticks(from min: Double, to max: Double, step: Double) -> [Double] {
        var ticks: [Double] = []
        var value = ceil(min / step) * step
        while value <= max + step * 0.01 {
            ticks.append(value)
            value += step
        }
        return ticks
    }

    static func niceStep(_ value: Double) -> Double {
        guard value > 0 else { return 1 }
        let exponent = floor(log10(value))
        let fraction = value / pow(10, exponent)
        let niceFraction: Double
        if fraction <= 1 {
            niceFraction = 1
        } else if fraction <= 2 {
            niceFraction = 2
        } else if fraction <= 5 {
            niceFraction = 5
        } else {
            niceFraction = 10
        }
        return niceFraction * pow(10, exponent)
    }
}
