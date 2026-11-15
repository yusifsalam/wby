import SwiftUI

/// One line of a `DayLineChart`.
struct DayChartSeries {
    let value: (HourlyForecast) -> Double?
    let stroke: AnyShapeStyle
    var lineWidth: CGFloat = 4
    var fill: AnyShapeStyle?
    var dashesPast = true
    var marksExtremes = false
    var tracksSelection = false
}

/// Hourly series for one local day over a full 00–24 axis, with a marker per
/// even hour along the top. On today, lines are dashed and dimmed before now
/// and a dot marks the current value. Dragging sideways selects an hour.
struct DayLineChart<HourMarker: View>: View {
    let detail: DayDetail
    let series: [DayChartSeries]
    let scale: ChartScale
    let axisLabel: (Double) -> String
    @Binding var selectedHour: HourlyForecast?
    let hourMarker: (HourlyForecast) -> HourMarker

    @State private var scrubbing: Bool?

    private let axisWidth: CGFloat = 36
    private let plotHeight: CGFloat = 180

    init(
        detail: DayDetail,
        series: [DayChartSeries],
        scale: ChartScale,
        axisLabel: @escaping (Double) -> String,
        selectedHour: Binding<HourlyForecast?>,
        @ViewBuilder hourMarker: @escaping (HourlyForecast) -> HourMarker
    ) {
        self.detail = detail
        self.series = series
        self.scale = scale
        self.axisLabel = axisLabel
        self._selectedHour = selectedHour
        self.hourMarker = hourMarker
    }

    var body: some View {
        VStack(spacing: 6) {
            HStack(spacing: 0) {
                markerRow
                    .frame(height: 18)
                Color.clear.frame(width: axisWidth, height: 1)
            }
            HStack(spacing: 0) {
                plot
                    .frame(height: plotHeight)
                axisLabels
                    .frame(width: axisWidth, height: plotHeight)
            }
            HStack(spacing: 0) {
                HourAxis()
                Color.clear.frame(width: axisWidth, height: 1)
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

    private func points(_ series: DayChartSeries) -> [Point] {
        detail.hours.compactMap { hour in
            series.value(hour).map { Point(x: xFraction(hour), value: $0) }
        }
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

    // MARK: - Marker row

    private var markerRow: some View {
        GeometryReader { geo in
            ForEach(detail.hours.filter { detail.localHour(of: $0.time) % 2 == 0 }) { hour in
                hourMarker(hour)
                    .opacity(isPast(hour) ? 0.4 : 1)
                    .position(x: xFraction(hour) * geo.size.width, y: geo.size.height / 2)
            }
        }
    }

    // MARK: - Plot

    private var plot: some View {
        GeometryReader { geo in
            let width = geo.size.width
            let height = geo.size.height
            let nowX = detail.nowFraction.map { CGFloat($0) * width }
            let tracked = series.first { $0.tracksSelection }.map(points) ?? []
            ZStack(alignment: .topLeading) {
                gridlines(width: width, height: height)
                ForEach(series.indices, id: \.self) { i in
                    seriesView(series[i], width: width, height: height, nowX: nowX)
                }
                if let nowX {
                    Rectangle()
                        .fill(.primary.opacity(0.3))
                        .frame(width: 1, height: height)
                        .position(x: nowX, y: height / 2)
                    if selectedHour == nil, let fraction = detail.nowFraction, let value = interpolate(tracked, at: fraction) {
                        marker(at: CGPoint(x: nowX, y: scale.y(value, height: height)))
                    }
                }
                ForEach(series.indices.filter { series[$0].marksExtremes }, id: \.self) { i in
                    extremes(points(series[i]), width: width, height: height)
                }
                if let selectedHour {
                    let x = xFraction(selectedHour) * width
                    Rectangle()
                        .fill(.primary.opacity(0.6))
                        .frame(width: 1.5, height: height)
                        .position(x: x, y: height / 2)
                    if let value = series.first(where: { $0.tracksSelection })?.value(selectedHour) {
                        marker(at: CGPoint(x: x, y: scale.y(value, height: height)))
                    }
                }
            }
            .contentShape(Rectangle())
            .simultaneousGesture(scrubGesture(width: width))
        }
    }

    @ViewBuilder
    private func seriesView(_ series: DayChartSeries, width: CGFloat, height: CGFloat, nowX: CGFloat?) -> some View {
        let points = points(series)
        let line = linePath(points, width: width, height: height)
        let solid = StrokeStyle(lineWidth: series.lineWidth, lineCap: .round, lineJoin: .round)
        let dashed = StrokeStyle(lineWidth: series.lineWidth, lineCap: .round, lineJoin: .round, dash: [series.lineWidth + 1, series.lineWidth + 3])
        if let fill = series.fill {
            let area = areaPath(points, width: width, height: height)
            if let nowX {
                area.fill(fill).opacity(0.45).clipped(from: 0, to: nowX)
                area.fill(fill).clipped(from: nowX, to: width)
            } else {
                area.fill(fill)
            }
        }
        if let nowX, series.dashesPast {
            line.stroke(series.stroke, style: dashed).opacity(0.6).clipped(from: 0, to: nowX)
            line.stroke(series.stroke, style: solid).clipped(from: nowX, to: width)
        } else {
            line.stroke(series.stroke, style: solid)
        }
    }

    private func linePath(_ points: [Point], width: CGFloat, height: CGFloat) -> Path {
        Path { path in
            for (i, point) in points.enumerated() {
                let p = CGPoint(x: CGFloat(point.x) * width, y: scale.y(point.value, height: height))
                if i == 0 { path.move(to: p) } else { path.addLine(to: p) }
            }
        }
    }

    private func areaPath(_ points: [Point], width: CGFloat, height: CGFloat) -> Path {
        var path = linePath(points, width: width, height: height)
        if let first = points.first, let last = points.last {
            path.addLine(to: CGPoint(x: CGFloat(last.x) * width, y: height))
            path.addLine(to: CGPoint(x: CGFloat(first.x) * width, y: height))
            path.closeSubpath()
        }
        return path
    }

    private func gridlines(width: CGFloat, height: CGFloat) -> some View {
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
            .frame(width: 10, height: 10)
            .overlay(Circle().stroke(.black.opacity(0.25), lineWidth: 1))
            .position(point)
    }

    @ViewBuilder
    private func extremes(_ points: [Point], width: CGFloat, height: CGFloat) -> some View {
        if let high = points.max(by: { $0.value < $1.value }),
           let low = points.min(by: { $0.value < $1.value }),
           high.value - low.value >= 1 {
            extremeDot(high, width: width, height: height)
            extremeDot(low, width: width, height: height)
            extremeLabel("H", at: high, width: width, y: max(scale.y(high.value, height: height) - 13, 6))
            extremeLabel("L", at: low, width: width, y: min(scale.y(low.value, height: height) + 13, height - 6))
        }
    }

    private func extremeDot(_ point: Point, width: CGFloat, height: CGFloat) -> some View {
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

    private var axisLabels: some View {
        GeometryReader { geo in
            ForEach(scale.ticks, id: \.self) { tick in
                Text(axisLabel(tick))
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

private extension View {
    func clipped(from start: CGFloat, to end: CGFloat) -> some View {
        mask(alignment: .leading) {
            Rectangle()
                .frame(width: max(end - start, 0))
                .offset(x: start)
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
    DayLineChart(
        detail: detail,
        series: [DayChartSeries(value: \.temperature, stroke: AnyShapeStyle(.orange), marksExtremes: true, tracksSelection: true)],
        scale: ChartScale.covering(-5, 10, divisions: 5),
        axisLabel: { "\($0.displayText())°" },
        selectedHour: $selected
    ) { _ in
        Image(systemName: "circle.fill")
            .font(.system(size: 6))
    }
    .padding()
}
