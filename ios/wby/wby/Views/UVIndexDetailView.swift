import SwiftUI

/// The hourly UV index as a histogram per local day, with a day strip for
/// switching days. FMI's UV forecast covers today and tomorrow only.
struct UVIndexDetailView: View {
    let details: [DayDetail]
    let points: [UVPoint]
    let timeZone: TimeZone

    @State private var selection: String

    private static let axisWidth: CGFloat = 36

    init(details: [DayDetail], points: [UVPoint], timeZone: TimeZone) {
        self.details = details
        self.points = points
        self.timeZone = timeZone
        self._selection = State(initialValue: DayDetail.initialSelection(in: details))
    }

    var body: some View {
        DaySheetContainer(
            details: details,
            isLoadingHours: false,
            selection: $selection,
            isAvailable: { !dayPoints($0).isEmpty },
            unavailableMessage: "FMI's UV forecast covers only today and tomorrow."
        ) { detail in
            VStack(alignment: .leading, spacing: 12) {
                headline(detail)
                chart(bars(detail))
            }
            if let summary = summary(detail) {
                DetailSection(title: "Daily Summary") {
                    Text(summary)
                        .font(.subheadline)
                }
            }
            DetailSection(title: "About the UV Index") {
                Text("The UV index measures the strength of the sun's ultraviolet radiation on the World Health Organization's scale. From 3 (Moderate) up, the WHO recommends shade around midday, covering clothing, a hat, and sunscreen. In Finland it seldom goes above 6 and stays low from October to March. The forecast is FMI's; hours it doesn't cover count as 0.")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }
        }
        .navigationTitle("UV Index")
        .navigationBarTitleDisplayMode(.inline)
    }

    // MARK: - Day data

    private func dayPoints(_ detail: DayDetail) -> [UVPoint] {
        guard let date = detail.date else { return [] }
        return UVIndex.dayPoints(points, timeZone: timeZone, day: date)
    }

    private func bars(_ detail: DayDetail) -> [UVIndex.DayBar] {
        guard let date = detail.date else { return [] }
        return UVIndex.dayBars(points, timeZone: timeZone, day: date)
    }

    // MARK: - Headline

    private struct HeadlineValues {
        let value: Double
        let caption: String
        var time: String?
    }

    private func headline(_ detail: DayDetail) -> some View {
        let values = headlineValues(detail)
        return VStack(alignment: .leading, spacing: 2) {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Text("\(Int(values.value.rounded()))")
                    .font(.system(size: 34, weight: .semibold))
                    .monospacedDigit()
                Text(UVIndex.level(for: values.value))
                    .font(.title3.weight(.semibold))
                    .foregroundStyle(.secondary)
                Spacer()
                if let time = values.time {
                    Text(time)
                        .font(.subheadline.weight(.semibold))
                        .foregroundStyle(.secondary)
                        .monospacedDigit()
                }
            }
            Text(values.caption)
                .font(.subheadline)
                .foregroundStyle(.secondary)
        }
    }

    private func headlineValues(_ detail: DayDetail) -> HeadlineValues {
        let day = dayPoints(detail)
        let peak = UVIndex.peak(day)
        guard detail.isToday else {
            guard let peak, peak.value >= 0.5 else {
                return HeadlineValues(value: peak?.value ?? 0, caption: "Low levels all day")
            }
            return HeadlineValues(
                value: peak.value,
                caption: "Peak, around \(Self.timeLabel(peak.time, timeZone: timeZone))"
            )
        }

        let now = Date()
        // Hours the forecast misses (the first local hours before FMI's 00:00
        // UTC start) count as 0, like the bars.
        let current = bars(detail).first { $0.current }?.uv ?? 0
        let remaining = day.filter { $0.time >= now.addingTimeInterval(-3600) }
        let outlook: String
        if let remainingPeak = UVIndex.peak(remaining), remainingPeak.value > current {
            outlook = "Rising to \(Int(remainingPeak.value.rounded())) · \(UVIndex.level(for: remainingPeak.value)) at \(Self.timeLabel(remainingPeak.time, timeZone: timeZone))"
        } else if let peak, peak.value > 0 {
            outlook = current == 0 ? "Low levels for the rest of the day" : "Highest for the rest of the day"
        } else {
            outlook = "Low levels all day"
        }
        return HeadlineValues(
            value: current,
            caption: outlook,
            time: "Now, \(Self.timeLabel(now, timeZone: timeZone))"
        )
    }

    // MARK: - Summary

    private func summary(_ detail: DayDetail) -> String? {
        let day = dayPoints(detail).filter { $0.uv.isFinite }
        guard let peak = UVIndex.peak(day) else { return nil }
        guard peak.value >= 0.5 else {
            return "\(detail.summaryLead), the UV index stays at 0."
        }
        var text = "\(detail.summaryLead), the UV index peaks at \(Int(peak.value.rounded())) (\(UVIndex.level(for: peak.value).lowercased())) around \(Self.timeLabel(peak.time, timeZone: timeZone))."
        let protected = day.filter { $0.uv >= 3 }
        if let first = protected.first, let last = protected.last {
            text += " Use sun protection from \(Self.timeLabel(first.time, timeZone: timeZone)) to \(Self.timeLabel(last.time.addingTimeInterval(3600), timeZone: timeZone))."
        } else {
            text += " No sun protection is needed."
        }
        return text
    }

    // MARK: - Chart

    private var boundaries: [Double] {
        UVIndex.bands.map { min($0.limit, UVIndex.scaleMax) }
    }

    private func chart(_ bars: [UVIndex.DayBar]) -> some View {
        VStack(spacing: 6) {
            HStack(spacing: 0) {
                plot(bars)
                    .frame(height: 180)
                scaleAxis
                    .frame(width: Self.axisWidth, height: 180)
            }
            HStack(spacing: 0) {
                HourAxis()
                Color.clear.frame(width: Self.axisWidth)
            }
        }
    }

    private func plot(_ bars: [UVIndex.DayBar]) -> some View {
        GeometryReader { geo in
            let width = geo.size.width
            let height = geo.size.height
            let slot = width / 24
            let barWidth = slot * 0.72
            ZStack(alignment: .topLeading) {
                Path { path in
                    for value in boundaries {
                        let y = height * (1 - UVIndex.barFraction(value))
                        path.move(to: CGPoint(x: 0, y: y))
                        path.addLine(to: CGPoint(x: width, y: y))
                    }
                }
                .stroke(.primary.opacity(0.08), lineWidth: 1)

                ForEach(Array(UVIndex.bands.enumerated()), id: \.offset) { i, band in
                    let from = i == 0 ? 0 : boundaries[i - 1]
                    let to = boundaries[i]
                    let mid = from == to ? to : (from + to) / 2
                    Text(band.label)
                        .font(.system(size: 9, weight: .semibold))
                        .foregroundStyle(.tertiary)
                        .position(x: 24, y: height * (1 - UVIndex.barFraction(mid)))
                }

                ForEach(bars) { bar in
                    let x = slot * (CGFloat(bar.hour) + 0.5)
                    if let uv = bar.uv {
                        let barHeight = max(height * UVIndex.barFraction(uv), 2)
                        RoundedRectangle(cornerRadius: 2, style: .continuous)
                            .fill(UVIndex.color(for: uv))
                            .frame(width: barWidth, height: barHeight)
                            .position(x: x, y: height - barHeight / 2)
                            .opacity(bar.past ? 0.35 : 1)
                        if uv >= 0.5 {
                            Text("\(Int(uv.rounded()))")
                                .font(.system(size: 8, weight: .semibold))
                                .foregroundStyle(bar.past ? .tertiary : .secondary)
                                .position(x: x, y: max(height - barHeight - 7, 5))
                        }
                    }
                    if bar.current {
                        Rectangle()
                            .fill(.primary.opacity(0.35))
                            .frame(width: 1, height: height)
                            .position(x: x, y: height / 2)
                    }
                }
            }
        }
    }

    private var scaleAxis: some View {
        GeometryReader { geo in
            ForEach(0...Int(UVIndex.scaleMax), id: \.self) { value in
                let major = value == 0 || boundaries.contains(Double(value))
                let y = geo.size.height * (1 - UVIndex.barFraction(Double(value)))
                Text("\(value)")
                    .font(.caption2)
                    .foregroundStyle(major ? .secondary : .quaternary)
                    .position(x: geo.size.width / 2 + 4, y: min(max(y, 5), geo.size.height - 5))
            }
        }
    }

    private static func timeLabel(_ date: Date, timeZone: TimeZone) -> String {
        let formatter = timeFormatter
        formatter.timeZone = timeZone
        return formatter.string(from: date)
    }

    private static let timeFormatter: DateFormatter = {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "HH:mm"
        return f
    }()
}

#Preview {
    NavigationStack {
        UVIndexDetailView(
            details: DayDetail.build(days: PreviewData.makeDaily(), hours: [], timeZone: TimeZone(identifier: "Europe/Helsinki")!),
            points: PreviewData.makeUVForecast(),
            timeZone: TimeZone(identifier: "Europe/Helsinki")!
        )
    }
}
