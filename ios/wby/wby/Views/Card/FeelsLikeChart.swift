import CoreLocation
import SwiftUI

/// One local day of hourly temperature and feels-like: the selected metric in
/// temperature colors over the other as a faint reference line, with the
/// day's weather symbols along the top.
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

    private var scale: ChartScale {
        let values = detail.hours.flatMap { [$0.temperature, $0.feelsLike] }.compactMap { $0 }
        guard let low = values.min(), let high = values.max() else {
            return ChartScale(min: 0, max: 10, ticks: [0, 5, 10])
        }
        return ChartScale.covering(low, high, divisions: 5)
    }

    var body: some View {
        let scale = scale
        DayLineChart(
            detail: detail,
            series: [
                DayChartSeries(
                    value: metric.other.value,
                    stroke: AnyShapeStyle(.primary.opacity(0.22)),
                    lineWidth: 3,
                    dashesPast: false
                ),
                DayChartSeries(
                    value: metric.value,
                    stroke: AnyShapeStyle(TemperatureColor.verticalGradient(min: scale.min, max: scale.max)),
                    marksExtremes: true,
                    tracksSelection: true
                ),
            ],
            scale: scale,
            axisLabel: { "\($0.displayText())°" },
            selectedHour: $selectedHour
        ) { hour in
            Image(systemName: SmartSymbol.systemImageName(for: WeatherSymbols.nightAdjusted(
                hour.symbol,
                coordinate: coordinate,
                at: hour.time,
                timeZone: detail.timeZone,
                elevationMeters: elevationMeters
            )))
            .symbolRenderingMode(.multicolor)
            .font(.system(size: 12))
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
    .padding()
}
