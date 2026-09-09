import SwiftUI

/// Temperature and feels-like curves over a precipitation histogram for one
/// local day. The hour axis is always the full day, so today shows elapsed
/// hours as empty space and every day lines up the same way.
struct DayDetailChart: View {
    let detail: DayDetail

    private static let hourTicks = stride(from: 0, to: 24, by: 3).map { $0 }
    private static let axisWidth: CGFloat = 40

    private struct Sample {
        let x: Double
        let temperature: Double?
        let feelsLike: Double?
        let precipitation: Double
    }

    private var samples: [Sample] {
        detail.hours.map { hour in
            Sample(
                x: (Double(detail.localHour(of: hour.time)) + 0.5) / 24,
                temperature: hour.temperature,
                feelsLike: hour.feelsLike,
                precipitation: hour.precipitation1h ?? 0
            )
        }
    }

    var body: some View {
        let samples = samples
        let temperatureScale = temperatureScale(samples)
        let precipitationScale = precipitationScale(samples)
        VStack(alignment: .leading, spacing: 8) {
            if let temperatureScale {
                Text("Temperature")
                    .font(.subheadline)
                    .fontWeight(.medium)
                HStack(spacing: 0) {
                    temperaturePlot(samples, scale: temperatureScale)
                        .frame(height: 140)
                    axisLabels(temperatureScale.ticks, scale: temperatureScale) { "\(Int($0.rounded()))°" }
                        .frame(width: Self.axisWidth, height: 140)
                }
                HStack(spacing: 14) {
                    legendItem("Temperature", color: .orange, dashed: false)
                    legendItem("Feels like", color: .blue, dashed: true)
                }
                .padding(.bottom, 8)
            }

            Text("Precipitation")
                .font(.subheadline)
                .fontWeight(.medium)
            HStack(spacing: 0) {
                precipitationPlot(samples, scale: precipitationScale)
                    .frame(height: 80)
                axisLabels(precipitationScale.ticks, scale: precipitationScale) { DayDetail.formatPrecip($0) }
                    .frame(width: Self.axisWidth, height: 80)
            }
            HStack(spacing: 0) {
                hourAxis
                Color.clear.frame(width: Self.axisWidth)
            }
        }
    }

    // MARK: - Scales

    private struct Scale {
        let min: Double
        let max: Double
        let ticks: [Double]

        func y(_ value: Double, height: CGFloat) -> CGFloat {
            let range = Swift.max(max - min, 0.001)
            return height * (1 - CGFloat((value - min) / range))
        }
    }

    private func temperatureScale(_ samples: [Sample]) -> Scale? {
        let temps = samples.compactMap(\.temperature)
        guard temps.count >= 2 else { return nil }
        let values = temps + samples.compactMap(\.feelsLike)
        let min = floor(values.min()!) - 1
        let max = ceil(values.max()!) + 1
        let step = Self.niceStep((max - min) / 4)
        return Scale(min: min, max: max, ticks: Self.ticks(from: min, to: max, step: step))
    }

    /// Always renders, so a dry day reads as an empty 0–1 mm plot.
    private func precipitationScale(_ samples: [Sample]) -> Scale {
        let peak = samples.map(\.precipitation).max() ?? 0
        let step = Self.niceStep(Swift.max(peak, 1) / 2)
        let max = Swift.max(1, ceil(peak / step) * step)
        return Scale(min: 0, max: max, ticks: Self.ticks(from: step, to: max, step: step))
    }

    private static func ticks(from min: Double, to max: Double, step: Double) -> [Double] {
        var ticks: [Double] = []
        var value = ceil(min / step) * step
        while value <= max + step * 0.01 {
            ticks.append(value)
            value += step
        }
        return ticks
    }

    private static func niceStep(_ value: Double) -> Double {
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

    // MARK: - Plots

    private func temperaturePlot(_ samples: [Sample], scale: Scale) -> some View {
        GeometryReader { geo in
            let width = geo.size.width
            let height = geo.size.height
            ZStack(alignment: .topLeading) {
                gridlines(scale.ticks, scale: scale, width: width, height: height)
                line(samples, pick: \.feelsLike, scale: scale, width: width, height: height)
                    .stroke(.blue.opacity(0.8), style: StrokeStyle(lineWidth: 1.5, dash: [4, 4]))
                line(samples, pick: \.temperature, scale: scale, width: width, height: height)
                    .stroke(.orange, style: StrokeStyle(lineWidth: 2.5, lineJoin: .round))
                nowMarker(width: width, height: height)
            }
        }
    }

    private func precipitationPlot(_ samples: [Sample], scale: Scale) -> some View {
        GeometryReader { geo in
            let width = geo.size.width
            let height = geo.size.height
            let barWidth = width / 24 * 0.7
            ZStack(alignment: .topLeading) {
                gridlines(scale.ticks, scale: scale, width: width, height: height)
                ForEach(Array(samples.enumerated()), id: \.offset) { _, sample in
                    if sample.precipitation > 0 {
                        let top = scale.y(sample.precipitation, height: height)
                        RoundedRectangle(cornerRadius: 2, style: .continuous)
                            .fill(.blue.opacity(0.85))
                            .frame(width: barWidth, height: Swift.max(height - top, 2))
                            .position(x: CGFloat(sample.x) * width, y: height - Swift.max(height - top, 2) / 2)
                    }
                }
                nowMarker(width: width, height: height)
            }
        }
    }

    private func line(_ samples: [Sample], pick: KeyPath<Sample, Double?>, scale: Scale, width: CGFloat, height: CGFloat) -> Path {
        Path { path in
            var started = false
            for sample in samples {
                guard let value = sample[keyPath: pick] else { continue }
                let point = CGPoint(x: CGFloat(sample.x) * width, y: scale.y(value, height: height))
                if started {
                    path.addLine(to: point)
                } else {
                    path.move(to: point)
                    started = true
                }
            }
        }
    }

    private func gridlines(_ ticks: [Double], scale: Scale, width: CGFloat, height: CGFloat) -> some View {
        Path { path in
            for tick in ticks {
                let y = scale.y(tick, height: height)
                path.move(to: CGPoint(x: 0, y: y))
                path.addLine(to: CGPoint(x: width, y: y))
            }
        }
        .stroke(.primary.opacity(0.08), lineWidth: 1)
    }

    @ViewBuilder
    private func nowMarker(width: CGFloat, height: CGFloat) -> some View {
        if let fraction = detail.nowFraction {
            Rectangle()
                .fill(.primary.opacity(0.35))
                .frame(width: 1, height: height)
                .position(x: CGFloat(fraction) * width, y: height / 2)
        }
    }

    private func axisLabels(_ ticks: [Double], scale: Scale, format: @escaping (Double) -> String) -> some View {
        GeometryReader { geo in
            ForEach(ticks, id: \.self) { tick in
                let y = scale.y(tick, height: geo.size.height)
                Text(format(tick))
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
                    .position(x: geo.size.width / 2 + 4, y: min(max(y, 6), geo.size.height - 6))
            }
        }
    }

    private var hourAxis: some View {
        GeometryReader { geo in
            ForEach(Self.hourTicks, id: \.self) { hour in
                let x = (CGFloat(hour) + 0.5) / 24 * geo.size.width
                Text(String(format: "%02d", hour))
                    .font(.system(size: 9))
                    .foregroundStyle(hour % 6 == 0 ? .secondary : .tertiary)
                    .position(x: x, y: 6)
            }
        }
        .frame(height: 12)
    }

    private func legendItem(_ title: String, color: Color, dashed: Bool) -> some View {
        HStack(spacing: 6) {
            Path { path in
                path.move(to: CGPoint(x: 0, y: 1))
                path.addLine(to: CGPoint(x: 18, y: 1))
            }
            .stroke(color, style: StrokeStyle(lineWidth: 2, dash: dashed ? [3, 3] : []))
            .frame(width: 18, height: 2)
            Text(title)
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
    }
}

#Preview {
    let detail = DayDetail.build(
        days: [PreviewData.makeDaily()[0]],
        hours: PreviewData.makeFullDay(),
        timeZone: TimeZone(identifier: "Europe/Helsinki")!
    )[0]
    DayDetailChart(detail: detail)
        .weatherCard()
        .padding()
}
