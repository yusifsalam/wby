import CoreLocation
import SwiftUI

/// One local day of hourly temperature and feels-like: the selected metric in
/// temperature colors, the other as a faint reference line, and the day's
/// weather symbols along the top. Dragging sideways across the plot selects
/// an hour.
struct FeelsLikeChart: View {
    enum Metric: String, CaseIterable, Identifiable {
        case actual
        case feelsLike

        var id: Self { self }

        var title: String {
            switch self {
            case .actual: "Actual"
            case .feelsLike: "Feels Like"
            }
        }

        var other: Metric { self == .actual ? .feelsLike : .actual }

        func value(_ hour: HourlyForecast) -> Double? {
            self == .actual ? hour.temperature : hour.feelsLike
        }
    }

    let detail: DayDetail
    let metric: Metric
    let coordinate: CLLocationCoordinate2D
    let elevationMeters: Double
    @Binding var selectedHour: HourlyForecast?

    @State private var scrubbing: Bool?

    private static let axisWidth: CGFloat = 36
    private static let plotHeight: CGFloat = 180

    var body: some View {
        let scale = scale
        VStack(spacing: 6) {
            HStack(spacing: 0) {
                symbolRow
                    .frame(height: 18)
                Color.clear.frame(width: Self.axisWidth, height: 1)
            }
            HStack(spacing: 0) {
                plot(scale)
                    .frame(height: Self.plotHeight)
                axisLabels(scale)
                    .frame(width: Self.axisWidth, height: Self.plotHeight)
            }
            HStack(spacing: 0) {
                HourAxis()
                Color.clear.frame(width: Self.axisWidth, height: 1)
            }
        }
    }

    // MARK: - Geometry

    private struct Point {
        let x: Double
        let value: Double
    }

    private func xFraction(_ hour: HourlyForecast) -> Double {
        (Double(detail.localHour(of: hour.time)) + 0.5) / 24
    }

    private func points(_ metric: Metric) -> [Point] {
        detail.hours.compactMap { hour in
            metric.value(hour).map { Point(x: xFraction(hour), value: $0) }
        }
    }

    private var scale: ChartScale {
        let values = detail.hours.flatMap { [$0.temperature, $0.feelsLike] }.compactMap { $0 }
        guard let low = values.min(), let high = values.max() else {
            return ChartScale(min: 0, max: 10, ticks: [0, 5, 10])
        }
        return ChartScale.covering(low, high, divisions: 5)
    }

    private func interpolate(_ points: [Point], at x: Double) -> Double? {
        guard let first = points.first, let last = points.last else { return nil }
        if x <= first.x { return first.value }
        for (a, b) in zip(points, points.dropFirst()) where x <= b.x {
            let t = b.x > a.x ? (x - a.x) / (b.x - a.x) : 1
            return a.value + (b.value - a.value) * t
        }
        return last.value
    }

    private func isPast(_ hour: HourlyForecast) -> Bool {
        hour.time.addingTimeInterval(3600) <= Date()
    }

    // MARK: - Symbols

    private var symbolRow: some View {
        GeometryReader { geo in
            ForEach(detail.hours.filter { detail.localHour(of: $0.time) % 2 == 0 }) { hour in
                Image(systemName: SmartSymbol.systemImageName(for: WeatherSymbols.nightAdjusted(
                    hour.symbol,
                    coordinate: coordinate,
                    at: hour.time,
                    timeZone: detail.timeZone,
                    elevationMeters: elevationMeters
                )))
                .symbolRenderingMode(.multicolor)
                .font(.system(size: 12))
                .opacity(isPast(hour) ? 0.4 : 1)
                .position(x: xFraction(hour) * geo.size.width, y: geo.size.height / 2)
            }
        }
    }

    // MARK: - Plot

    private func plot(_ scale: ChartScale) -> some View {
        GeometryReader { geo in
            let width = geo.size.width
            let height = geo.size.height
            let primary = points(metric)
            let primaryPath = path(primary, scale: scale, width: width, height: height)
            let gradient = TemperatureColor.verticalGradient(min: scale.min, max: scale.max)
            let solid = StrokeStyle(lineWidth: 4, lineCap: .round, lineJoin: .round)
            let dashed = StrokeStyle(lineWidth: 4, lineCap: .round, lineJoin: .round, dash: [5, 7])
            let nowX = detail.nowFraction.map { CGFloat($0) * width }
            ZStack(alignment: .topLeading) {
                gridlines(scale, width: width, height: height)
                path(points(metric.other), scale: scale, width: width, height: height)
                    .stroke(.primary.opacity(0.22), style: StrokeStyle(lineWidth: 3, lineCap: .round, lineJoin: .round))
                if let nowX {
                    primaryPath
                        .stroke(gradient, style: dashed)
                        .opacity(0.6)
                        .mask(alignment: .leading) {
                            Rectangle().frame(width: max(nowX, 0))
                        }
                    primaryPath
                        .stroke(gradient, style: solid)
                        .mask(alignment: .leading) {
                            Rectangle()
                                .frame(width: max(width - nowX, 0))
                                .offset(x: nowX)
                        }
                    Rectangle()
                        .fill(.primary.opacity(0.3))
                        .frame(width: 1, height: height)
                        .position(x: nowX, y: height / 2)
                    if selectedHour == nil, let fraction = detail.nowFraction, let value = interpolate(primary, at: fraction) {
                        marker(at: CGPoint(x: nowX, y: scale.y(value, height: height)))
                    }
                } else {
                    primaryPath.stroke(gradient, style: solid)
                }
                extremes(primary, scale: scale, width: width, height: height)
                if let selectedHour {
                    let x = xFraction(selectedHour) * width
                    Rectangle()
                        .fill(.primary.opacity(0.5))
                        .frame(width: 1, height: height)
                        .position(x: x, y: height / 2)
                    if let value = metric.value(selectedHour) {
                        marker(at: CGPoint(x: x, y: scale.y(value, height: height)))
                    }
                }
            }
            .contentShape(Rectangle())
            .simultaneousGesture(scrubGesture(width: width))
        }
    }

    private func path(_ points: [Point], scale: ChartScale, width: CGFloat, height: CGFloat) -> Path {
        Path { path in
            for (i, point) in points.enumerated() {
                let p = CGPoint(x: CGFloat(point.x) * width, y: scale.y(point.value, height: height))
                if i == 0 { path.move(to: p) } else { path.addLine(to: p) }
            }
        }
    }

    private func gridlines(_ scale: ChartScale, width: CGFloat, height: CGFloat) -> some View {
        Path { path in
            for tick in scale.ticks {
                let y = scale.y(tick, height: height)
                path.move(to: CGPoint(x: 0, y: y))
                path.addLine(to: CGPoint(x: width, y: y))
            }
        }
        .stroke(.primary.opacity(0.08), lineWidth: 1)
    }

    private func marker(at point: CGPoint) -> some View {
        Circle()
            .fill(.white)
            .frame(width: 9, height: 9)
            .overlay(Circle().stroke(.black.opacity(0.25), lineWidth: 1))
            .position(point)
    }

    @ViewBuilder
    private func extremes(_ points: [Point], scale: ChartScale, width: CGFloat, height: CGFloat) -> some View {
        if let high = points.max(by: { $0.value < $1.value }),
           let low = points.min(by: { $0.value < $1.value }),
           high.value - low.value >= 1 {
            extremeDot(high, scale: scale, width: width, height: height)
            extremeDot(low, scale: scale, width: width, height: height)
            extremeLabel("H", at: high, width: width, y: max(scale.y(high.value, height: height) - 13, 6))
            extremeLabel("L", at: low, width: width, y: min(scale.y(low.value, height: height) + 13, height - 6))
        }
    }

    private func extremeDot(_ point: Point, scale: ChartScale, width: CGFloat, height: CGFloat) -> some View {
        Circle()
            .fill(.black.opacity(0.45))
            .frame(width: 4, height: 4)
            .position(x: CGFloat(point.x) * width, y: scale.y(point.value, height: height))
    }

    private func extremeLabel(_ text: String, at point: Point, width: CGFloat, y: CGFloat) -> some View {
        Text(text)
            .font(.caption2.weight(.bold))
            .foregroundStyle(.secondary)
            .position(x: min(max(CGFloat(point.x) * width, 6), width - 6), y: y)
    }

    private func axisLabels(_ scale: ChartScale) -> some View {
        GeometryReader { geo in
            ForEach(scale.ticks, id: \.self) { tick in
                Text("\(tick.displayText())°")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .position(x: geo.size.width / 2 + 4, y: min(max(scale.y(tick, height: geo.size.height), 6), geo.size.height - 6))
            }
        }
    }

    // MARK: - Scrubbing

    private func scrubGesture(width: CGFloat) -> some Gesture {
        DragGesture(minimumDistance: 6)
            .onChanged { value in
                if scrubbing == nil {
                    scrubbing = abs(value.translation.width) > abs(value.translation.height)
                }
                guard scrubbing == true else { return }
                let fraction = Double(min(max(value.location.x / max(width, 1), 0), 1))
                let hour = detail.hours.min { abs(xFraction($0) - fraction) < abs(xFraction($1) - fraction) }
                if hour?.time != selectedHour?.time {
                    selectedHour = hour
                }
            }
            .onEnded { _ in
                scrubbing = nil
                selectedHour = nil
            }
    }
}

#Preview {
    @Previewable @State var selected: HourlyForecast?
    let detail = DayDetail.build(
        days: [PreviewData.makeDaily()[0]],
        hours: PreviewData.makeFullDay(),
        timeZone: TimeZone(identifier: "Europe/Helsinki")!
    )[0]
    FeelsLikeChart(
        detail: detail,
        metric: .feelsLike,
        coordinate: CLLocationCoordinate2D(latitude: 60.1699, longitude: 24.9384),
        elevationMeters: 0,
        selectedHour: $selected
    )
    .weatherCard()
    .padding()
}
