import SwiftUI

/// Today's hourly UV index as a histogram in local time, headlined by the
/// current hour's value and the rest of the day's outlook.
struct UVIndexDetailView: View {
    let points: [UVPoint]
    let timeZone: TimeZone

    private static let hourTicks = stride(from: 0, to: 24, by: 3).map { $0 }
    private static let axisWidth: CGFloat = 24

    private var now: Date { Date() }
    private var today: [UVPoint] { UVIndex.dayPoints(points, timeZone: timeZone, now: now) }
    private var bars: [UVIndex.DayBar] { UVIndex.dayBars(points, timeZone: timeZone, now: now) }
    private var peak: UVIndex.Peak? { UVIndex.peak(today) }
    private var peakTomorrow: UVIndex.Peak? { UVIndex.peak(UVIndex.dayPoints(points, timeZone: timeZone, now: now, offsetDays: 1)) }

    /// Hours the forecast misses (the first local hours before FMI's 00:00
    /// UTC start) count as 0, like the bars.
    private var current: Double {
        bars.first { $0.current }?.uv ?? 0
    }

    private var outlook: String {
        let remaining = today.filter { $0.time >= now.addingTimeInterval(-3600) }
        if let remainingPeak = UVIndex.peak(remaining), remainingPeak.value > current {
            return "Rising to \(Int(remainingPeak.value.rounded())) · \(UVIndex.level(for: remainingPeak.value)) at \(Self.timeLabel(remainingPeak.time, timeZone: timeZone))"
        }
        guard let peak, peak.value > 0 else { return "Low levels all day" }
        return current == 0 ? "Low levels for the rest of the day" : "Highest for the rest of the day"
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                headline
                    .weatherCard()
                VStack(alignment: .leading, spacing: 12) {
                    Text("Today, by hour")
                        .font(.subheadline)
                        .fontWeight(.medium)
                    chart
                }
                .weatherCard()
                Text("Hourly UV index forecast from FMI for the local day. Bars show each hour's value; hours the forecast doesn't cover count as 0.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            .padding()
        }
        .navigationTitle("UV Index")
        .navigationBarTitleDisplayMode(.inline)
    }

    // MARK: - Headline

    private var headline: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Label("UV INDEX", systemImage: "sun.max")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Spacer()
                Text("Now, \(Self.timeLabel(now, timeZone: timeZone))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Text("\(Int(current.rounded()))")
                    .font(.system(size: 44, weight: .bold))
                Text(UVIndex.level(for: current))
                    .font(.title3)
                    .fontWeight(.semibold)
            }
            Text(outlook)
                .font(.subheadline)
                .foregroundStyle(.secondary)
            if let peakTomorrow {
                Text("Tomorrow up to \(Int(peakTomorrow.value.rounded())) · \(UVIndex.level(for: peakTomorrow.value))")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }
        }
    }

    // MARK: - Chart

    private var boundaries: [Double] {
        UVIndex.bands.map { min($0.limit, UVIndex.scaleMax) }
    }

    private var chart: some View {
        VStack(spacing: 6) {
            HStack(spacing: 0) {
                plot
                    .frame(height: 180)
                scaleAxis
                    .frame(width: Self.axisWidth, height: 180)
            }
            HStack(spacing: 0) {
                hourAxis
                Color.clear.frame(width: Self.axisWidth)
            }
        }
    }

    private var plot: some View {
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
                    .font(.system(size: 9))
                    .foregroundStyle(major ? .secondary : .quaternary)
                    .position(x: geo.size.width / 2 + 4, y: min(max(y, 5), geo.size.height - 5))
            }
        }
    }

    private var hourAxis: some View {
        GeometryReader { geo in
            ForEach(Self.hourTicks, id: \.self) { hour in
                Text(String(format: "%02d", hour))
                    .font(.system(size: 9))
                    .foregroundStyle(hour % 6 == 0 ? .secondary : .tertiary)
                    .position(x: (CGFloat(hour) + 0.5) / 24 * geo.size.width, y: 6)
            }
        }
        .frame(height: 12)
    }

    private static func timeLabel(_ date: Date, timeZone: TimeZone) -> String {
        let formatter = timeFormatter
        formatter.timeZone = timeZone
        return formatter.string(from: date)
    }

    private static let timeFormatter: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "HH:mm"
        return f
    }()
}

#Preview {
    NavigationStack {
        UVIndexDetailView(points: PreviewData.makeUVForecast(), timeZone: TimeZone(identifier: "Europe/Helsinki")!)
    }
}
