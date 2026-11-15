import CoreLocation
import SwiftUI

/// The expanded feels-like card: a day strip, the day's hourly feels-like and
/// actual temperature curves with a toggle between them, a short summary, and
/// the hourly chance of precipitation.
struct FeelsLikeDetailView: View {
    let details: [DayDetail]
    let current: CurrentConditions
    let isLoadingHours: Bool
    let coordinate: CLLocationCoordinate2D
    let elevationMeters: Double

    @State private var selection: String
    @State private var metric: FeelsLikeChart.Metric = .feelsLike
    @State private var selectedHour: HourlyForecast?

    init(
        details: [DayDetail],
        current: CurrentConditions,
        isLoadingHours: Bool,
        coordinate: CLLocationCoordinate2D,
        elevationMeters: Double
    ) {
        self.details = details
        self.current = current
        self.isLoadingHours = isLoadingHours
        self.coordinate = coordinate
        self.elevationMeters = elevationMeters
        self._selection = State(initialValue: DayDetail.initialSelection(in: details))
    }

    var body: some View {
        DaySheetContainer(details: details, isLoadingHours: isLoadingHours, selection: $selection) { detail in
            content(detail)
        }
        .navigationTitle("Feels Like")
        .navigationBarTitleDisplayMode(.inline)
        .onChange(of: selection) { selectedHour = nil }
    }

    @ViewBuilder
    private func content(_ detail: DayDetail) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            headline(detail)
            FeelsLikeChart(
                detail: detail,
                metric: metric,
                coordinate: coordinate,
                elevationMeters: elevationMeters,
                selectedHour: $selectedHour
            )
        }

        Picker("Metric", selection: $metric.animation(.snappy)) {
            ForEach(FeelsLikeChart.Metric.allCases) { metric in
                Text(metric.title).tag(metric)
            }
        }
        .pickerStyle(.segmented)

        Text(metric == .feelsLike
            ? "What the temperature feels like, as a result of wind, humidity, or sunlight."
            : "The air temperature, as a thermometer in the shade reads it.")
            .font(.footnote)
            .foregroundStyle(.secondary)

        if let summary = summary(detail) {
            DetailSection(title: "Daily Summary") {
                Text(summary)
                    .font(.subheadline)
            }
        }

        DetailSection(title: "Chance of Precipitation") {
            Text("\(chanceLead(detail)): \(DayDetail.formatPercent(detail.popMax ?? 0))")
                .font(.subheadline)
                .foregroundStyle(.secondary)
            PrecipitationChanceChart(detail: detail)
                .padding(.top, 4)
        }

        DetailSection(title: "About the Feels-Like Temperature") {
            Text("The feels-like temperature describes how the air feels on exposed skin. Wind carries heat away and makes cold weather feel colder, while humidity and sunshine can make warm weather feel warmer. On a windy winter day it can be several degrees below the actual temperature.")
                .font(.subheadline)
                .foregroundStyle(.secondary)
        }
    }

    // MARK: - Headline

    private func headline(_ detail: DayDetail) -> some View {
        let values = headlineValues(detail)
        return VStack(alignment: .leading, spacing: 2) {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                HStack(alignment: .firstTextBaseline, spacing: 6) {
                    Text(values.primary)
                    if let low = values.low {
                        Text(low)
                            .foregroundStyle(.secondary)
                    }
                }
                .font(.system(size: 34, weight: .semibold))
                .monospacedDigit()
                .contentTransition(.numericText())
                Image(systemName: SmartSymbol.systemImageName(for: values.symbol))
                    .symbolRenderingMode(.multicolor)
                    .font(.title2)
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
                .monospacedDigit()
        }
    }

    private struct HeadlineValues {
        let primary: String
        var low: String?
        let caption: String
        let symbol: String?
        var time: String?
    }

    private func headlineValues(_ detail: DayDetail) -> HeadlineValues {
        if let hour = selectedHour {
            return pointValues(
                actual: hour.temperature,
                feelsLike: hour.feelsLike,
                symbol: nightAdjusted(hour, in: detail),
                time: detail.timeLabel(hour.time)
            )
        }
        if detail.isToday {
            let now = Date()
            let forecast = detail.hours.last { $0.time <= now } ?? detail.hours.first
            return pointValues(
                actual: current.resolvedTemperature ?? forecast?.temperature,
                feelsLike: current.resolvedFeelsLike ?? forecast?.feelsLike,
                symbol: forecast.flatMap { nightAdjusted($0, in: detail) },
                time: nil
            )
        }
        let values = detail.hours.compactMap(metric.value)
        let high = values.max() ?? (metric == .actual ? detail.high : nil)
        let low = values.min() ?? (metric == .actual ? detail.low : nil)
        return HeadlineValues(
            primary: DayDetail.formatTemp(high),
            low: DayDetail.formatTemp(low),
            caption: metric.title,
            symbol: detail.day.symbol
        )
    }

    private func pointValues(actual: Double?, feelsLike: Double?, symbol: String?, time: String?) -> HeadlineValues {
        let value = metric == .actual ? actual : feelsLike
        let otherValue = metric == .actual ? feelsLike : actual
        return HeadlineValues(
            primary: DayDetail.formatTemp(value),
            caption: "\(metric.other.title): \(DayDetail.formatTemp(otherValue))",
            symbol: symbol,
            time: time
        )
    }

    private func nightAdjusted(_ hour: HourlyForecast, in detail: DayDetail) -> String? {
        WeatherSymbols.nightAdjusted(
            hour.symbol,
            coordinate: coordinate,
            at: hour.time,
            timeZone: detail.timeZone,
            elevationMeters: elevationMeters
        )
    }

    // MARK: - Summary

    private func chanceLead(_ detail: DayDetail) -> String {
        switch detail.shortTitle {
        case "Today": "Today's chance"
        case "Tomorrow": "Tomorrow's chance"
        default: "\(detail.shortTitle)'s chance"
        }
    }

    private func summary(_ detail: DayDetail) -> String? {
        let feels = detail.hours.compactMap(\.feelsLike)
        guard let low = feels.min(), let high = feels.max() else { return nil }
        let lowText = DayDetail.formatTemp(low)
        let highText = DayDetail.formatTemp(high)
        var text = lowText == highText
            ? "\(detail.summaryLead), it feels like about \(lowText) all day."
            : "\(detail.summaryLead), it feels like \(lowText) to \(highText)."

        let gaps = detail.hours.compactMap { hour -> (hour: HourlyForecast, gap: Double)? in
            guard let temperature = hour.temperature, let feelsLike = hour.feelsLike else { return nil }
            return (hour, temperature - feelsLike)
        }
        if let colder = gaps.max(by: { $0.gap < $1.gap }), colder.gap >= 2 {
            text += " Wind makes it feel up to \(Int(colder.gap.rounded()))° colder than the actual temperature, around \(detail.timeLabel(colder.hour.time))."
        } else if let warmer = gaps.min(by: { $0.gap < $1.gap }), warmer.gap <= -2 {
            text += " It feels up to \(Int((-warmer.gap).rounded()))° warmer than the actual temperature, around \(detail.timeLabel(warmer.hour.time))."
        } else if !gaps.isEmpty {
            text += " That's close to the actual temperature."
        }
        return text
    }
}

#Preview {
    let timeZone = TimeZone(identifier: "Europe/Helsinki")!
    NavigationStack {
        FeelsLikeDetailView(
            details: DayDetail.build(days: PreviewData.makeDaily(), hours: PreviewData.makeFullDay(), timeZone: timeZone),
            current: CurrentConditions(
                temperature: 7,
                feelsLike: -1,
                windSpeed: 9.0,
                windGust: 14.0,
                windDirection: 266.0,
                humidity: 84.0,
                pressure: 1012.0,
                observedAt: .now
            ),
            isLoadingHours: false,
            coordinate: CLLocationCoordinate2D(latitude: 60.1699, longitude: 24.9384),
            elevationMeters: 0
        )
    }
}
