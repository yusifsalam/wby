import SwiftUI

/// The expanded wind card: a day strip, the day's hourly wind speed and gusts
/// with direction arrows, a short summary, and the peak gust compared with a
/// neighboring day.
struct WindDetailView: View {
    let details: [DayDetail]
    let current: CurrentConditions
    let isLoadingHours: Bool

    @State private var selection: String
    @State private var selectedHour: HourlyForecast?

    init(details: [DayDetail], current: CurrentConditions, isLoadingHours: Bool) {
        self.details = details
        self.current = current
        self.isLoadingHours = isLoadingHours
        self._selection = State(initialValue: DayDetail.initialSelection(in: details))
    }

    var body: some View {
        DaySheetContainer(details: details, isLoadingHours: isLoadingHours, selection: $selection) { detail in
            content(detail)
        }
        .navigationTitle("Wind")
        .navigationBarTitleDisplayMode(.inline)
        .onChange(of: selection) { selectedHour = nil }
    }

    @ViewBuilder
    private func content(_ detail: DayDetail) -> some View {
        VStack(spacing: 12) {
            headline(detail)
            WindChart(detail: detail, selectedHour: $selectedHour)
            legend
        }

        if let summary = summary(detail) {
            DetailSection(title: "Daily Summary") {
                Text(summary)
                    .font(.subheadline)
            }
        }

        if let comparison = comparison(for: detail) {
            DetailSection(title: "Daily Comparison") {
                Text(comparison.text)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                let peak = max(comparison.selected.value, comparison.other.value, 1)
                VStack(spacing: 8) {
                    comparisonBar(comparison.selected, peak: peak, highlighted: true)
                    comparisonBar(comparison.other, peak: peak, highlighted: false)
                }
                .padding(.top, 4)
            }
        }

        DetailSection(title: "About Wind Speed and Gusts") {
            Text("Wind speed is the average over ten minutes, 10 meters above the ground. Gusts are the strongest bursts within that time, measured over a few seconds, so they can be much stronger than the average.")
                .font(.subheadline)
                .foregroundStyle(.secondary)
        }
    }

    private var legend: some View {
        HStack(spacing: 16) {
            legendItem("Wind", color: WindChart.speedColor)
            legendItem("Gusts", color: WindChart.gustColor)
            Spacer()
            Text("m/s")
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
    }

    private func legendItem(_ title: String, color: Color) -> some View {
        HStack(spacing: 6) {
            Capsule()
                .fill(color)
                .frame(width: 14, height: 4)
            Text(title)
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
    }

    // MARK: - Headline

    private struct HeadlineValues {
        let speed: String
        let direction: String?
        let caption: String
        let time: String?
    }

    private func headline(_ detail: DayDetail) -> some View {
        let values = headlineValues(detail)
        return VStack(spacing: 2) {
            Text(values.time ?? " ")
                .font(.caption.weight(.semibold))
                .foregroundStyle(.secondary)
                .monospacedDigit()
            HStack(alignment: .firstTextBaseline, spacing: 6) {
                Text(values.speed)
                    .font(.system(size: 34, weight: .semibold))
                    .contentTransition(.numericText())
                Text("m/s")
                    .font(.title3)
                    .foregroundStyle(.secondary)
                if let direction = values.direction {
                    Text(direction)
                        .font(.title3.weight(.semibold))
                }
            }
            .monospacedDigit()
            Text(values.caption)
                .font(.subheadline)
                .foregroundStyle(.secondary)
                .monospacedDigit()
        }
        .frame(maxWidth: .infinity)
    }

    private func headlineValues(_ detail: DayDetail) -> HeadlineValues {
        if let hour = selectedHour {
            return HeadlineValues(
                speed: Self.speed(hour.windSpeed),
                direction: hour.windDirection.map(WindDirection.abbreviation),
                caption: "Gusts: \(Self.speed(hour.windGust)) m/s",
                time: detail.timeLabel(hour.time)
            )
        }
        if detail.isToday {
            let now = Date()
            let forecast = detail.hours.last { $0.time <= now } ?? detail.hours.first
            return HeadlineValues(
                speed: Self.speed(current.resolvedWindSpeed ?? forecast?.windSpeed),
                direction: (current.resolvedWindDirection ?? forecast?.windDirection).map(WindDirection.abbreviation),
                caption: "Gusts: \(Self.speed(current.resolvedWindGust ?? forecast?.windGust)) m/s",
                time: nil
            )
        }
        let speeds = detail.hours.compactMap(\.windSpeed)
        let range: String
        if let low = speeds.min(), let high = speeds.max(), Self.speed(low) != Self.speed(high) {
            range = "\(Self.speed(low))–\(Self.speed(high))"
        } else {
            range = Self.speed(speeds.first ?? detail.windAvg)
        }
        return HeadlineValues(
            speed: range,
            direction: nil,
            caption: "Gusts up to \(Self.speed(detail.gustMax)) m/s",
            time: nil
        )
    }

    private static func speed(_ value: Double?) -> String {
        guard let value else { return "--" }
        return "\(Int(value.rounded()))"
    }

    // MARK: - Summary

    private func summary(_ detail: DayDetail) -> String? {
        let speeds = detail.hours.compactMap(\.windSpeed)
        guard let low = speeds.min(), let high = speeds.max() else { return nil }
        var sentences: [String] = []
        if detail.isToday, let speed = current.resolvedWindSpeed {
            if let direction = current.resolvedWindDirection, speed >= 0.5 {
                sentences.append("Wind is currently \(Self.speed(speed)) m/s from the \(WindDirection.name(direction)).")
            } else {
                sentences.append("Wind is currently \(Self.speed(speed)) m/s.")
            }
        }
        let range = Self.speed(low) == Self.speed(high)
            ? "around \(Self.speed(high)) m/s"
            : "\(Self.speed(low)) to \(Self.speed(high)) m/s"
        if let gust = detail.gustMax {
            sentences.append("\(detail.summaryLead), wind speeds are \(range), with gusts up to \(Self.speed(gust)) m/s.")
        } else {
            sentences.append("\(detail.summaryLead), wind speeds are \(range).")
        }
        return sentences.joined(separator: " ")
    }

    // MARK: - Comparison

    private struct ComparisonDay {
        let label: String
        let value: Double
    }

    private struct Comparison {
        let text: String
        let selected: ComparisonDay
        let other: ComparisonDay
    }

    /// Compares with the day before, or with tomorrow when the selected day is
    /// the first one the forecast has.
    private func comparison(for detail: DayDetail) -> Comparison? {
        guard let index = details.firstIndex(where: { $0.id == detail.id }) else { return nil }
        let otherIndex = index > 0 ? index - 1 : index + 1
        guard details.indices.contains(otherIndex),
              let value = detail.gustMax,
              let otherValue = details[otherIndex].gustMax else { return nil }
        let other = details[otherIndex]
        let relation: String
        if abs(value - otherValue) < 1 {
            relation = "similar to"
        } else {
            relation = value > otherValue ? "higher than" : "lower than"
        }
        return Comparison(
            text: "The peak gust speed \(detail.phraseName) is \(relation) \(other.phraseName).",
            selected: ComparisonDay(label: detail.shortTitle, value: value),
            other: ComparisonDay(label: other.shortTitle, value: otherValue)
        )
    }

    private func comparisonBar(_ day: ComparisonDay, peak: Double, highlighted: Bool) -> some View {
        HStack(spacing: 8) {
            GeometryReader { geo in
                RoundedRectangle(cornerRadius: 6, style: .continuous)
                    .fill(highlighted ? AnyShapeStyle(.primary) : AnyShapeStyle(.fill.secondary))
                    .frame(width: max(geo.size.width * day.value / peak, 72))
                    .overlay(alignment: .leading) {
                        Text(day.label)
                            .font(.caption.weight(.semibold))
                            .foregroundStyle(highlighted ? AnyShapeStyle(Color(uiColor: .systemBackground)) : AnyShapeStyle(.primary))
                            .padding(.leading, 8)
                    }
            }
            .frame(height: 24)
            Text("\(Self.speed(day.value)) m/s")
                .font(.subheadline.weight(.semibold))
                .monospacedDigit()
                .frame(width: 56, alignment: .trailing)
        }
    }
}

#Preview {
    let timeZone = TimeZone(identifier: "Europe/Helsinki")!
    NavigationStack {
        WindDetailView(
            details: DayDetail.build(days: PreviewData.makeDaily(), hours: PreviewData.makeFullDay(), timeZone: timeZone),
            current: CurrentConditions(
                temperature: 7,
                feelsLike: 2,
                windSpeed: 9.0,
                windGust: 16.0,
                windDirection: 210.0,
                humidity: 84.0,
                pressure: 1012.0,
                observedAt: .now
            ),
            isLoadingHours: false
        )
    }
}
