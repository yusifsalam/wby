import SwiftUI

/// One local day of hourly wind speed as a filled area with gusts as a line
/// above it, and the wind direction every two hours along the top.
struct WindChart: View {
    let detail: DayDetail
    @Binding var selectedHour: HourlyForecast?

    static let speedColor = Color(red: 0.20, green: 0.78, blue: 0.55)
    static let gustColor = Color(red: 0.85, green: 0.80, blue: 0.30)

    var body: some View {
        let peak = detail.hours.flatMap { [$0.windSpeed, $0.windGust] }.compactMap { $0 }.max() ?? 0
        DayLineChart(
            detail: detail,
            series: [
                DayChartSeries(
                    value: \.windGust,
                    stroke: AnyShapeStyle(Self.gustColor.opacity(0.7)),
                    lineWidth: 3
                ),
                DayChartSeries(
                    value: \.windSpeed,
                    stroke: AnyShapeStyle(Self.speedColor),
                    fill: AnyShapeStyle(LinearGradient(
                        colors: [Self.speedColor.opacity(0.45), Self.speedColor.opacity(0.05)],
                        startPoint: .top,
                        endPoint: .bottom
                    )),
                    tracksSelection: true
                ),
            ],
            scale: ChartScale.fromZero(to: max(peak, 8), divisions: 5),
            axisLabel: { "\(Int($0))" },
            selectedHour: $selectedHour
        ) { hour in
            if let direction = hour.windDirection {
                Image(systemName: "arrow.up")
                    .font(.system(size: 10, weight: .bold))
                    .foregroundStyle(.secondary)
                    .rotationEffect(.degrees(direction + 180))
            }
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
    WindChart(detail: detail, selectedHour: $selected)
        .padding()
}
