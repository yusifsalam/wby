import SwiftUI

struct DailyForecastRow: View {
    let forecast: DailyForecast
    let overallLow: Double
    let overallHigh: Double
    private let absoluteMinTemp = -40.0
    private let absoluteMaxTemp = 40.0
    private let minimumBarFraction = 0.03

    var body: some View {
        HStack {
            Text(dayName)
                .foregroundStyle(.primary)
                .frame(width: 64, alignment: .leading)

            Image(systemName: symbolName)
                .frame(width: 40)
                .symbolRenderingMode(.multicolor)

            Text(formatTemp(forecast.low))
                .foregroundStyle(.secondary)
                .frame(width: 36, alignment: .trailing)

            temperatureBar
                .frame(height: 4)

            Text(formatTemp(forecast.high))
                .foregroundStyle(.primary)
                .frame(width: 36, alignment: .trailing)
        }
        .padding(.vertical, 8)
    }

    private var dayName: String {
        guard let date = forecast.displayDate else { return forecast.date }
        if Calendar.current.isDateInToday(date) { return "Today" }
        let formatter = DateFormatter()
        formatter.dateFormat = "EEE"
        return formatter.string(from: date)
    }

    private var symbolName: String {
        SmartSymbol.systemImageName(for: forecast.symbol)
    }

    private var temperatureBar: some View {
        GeometryReader { geo in
            let lo = forecast.low ?? overallLow
            let hi = forecast.high ?? overallHigh
            let range = max(overallHigh - overallLow, 1)
            let startTemp = min(lo, hi)
            let endTemp = max(lo, hi)
            let startFraction = min(max((startTemp - overallLow) / range, 0), 1)
            let rawEndFraction = min(max((endTemp - overallLow) / range, 0), 1)
            let endFraction = max(rawEndFraction, min(startFraction + minimumBarFraction, 1))

            Capsule()
                .fill(Color.primary.opacity(0.16))
                .overlay(alignment: .leading) {
                    windowTemperatureGradient
                        .frame(width: geo.size.width, height: geo.size.height)
                        .mask(alignment: .leading) {
                            Capsule()
                                .frame(width: geo.size.width * (endFraction - startFraction))
                                .offset(x: geo.size.width * startFraction)
                        }
                }
        }
    }

    private var windowTemperatureGradient: LinearGradient {
        let low = min(overallLow, overallHigh)
        let high = max(overallLow, overallHigh)
        let mid = (low + high) / 2

        return LinearGradient(
            colors: [colorForTemperature(low), colorForTemperature(mid), colorForTemperature(high)],
            startPoint: .leading,
            endPoint: .trailing
        )
    }

    private func colorForTemperature(_ temp: Double) -> Color {
        let t = min(max(temp, absoluteMinTemp), absoluteMaxTemp)
        switch t {
        case ..<(-30): return Color(red: 0.23, green: 0.08, blue: 0.36)
        case ..<(-20): return Color(red: 0.27, green: 0.19, blue: 0.59)
        case ..<(-10): return Color(red: 0.20, green: 0.36, blue: 0.83)
        case ..<0: return Color(red: 0.20, green: 0.58, blue: 0.95)
        case ..<10: return Color(red: 0.22, green: 0.77, blue: 0.72)
        case ..<20: return Color(red: 0.94, green: 0.82, blue: 0.28)
        case ..<30: return Color(red: 0.93, green: 0.49, blue: 0.23)
        default: return Color(red: 0.57, green: 0.10, blue: 0.12)
        }
    }

    private func formatTemp(_ temp: Double?) -> String {
        guard let temp else { return "--" }
        return "\(Int(temp.rounded()))°"
    }
}

#Preview {
    VStack(spacing: 0) {
        ForEach(sampleForecasts) { day in
            DailyForecastRow(
                forecast: day,
                overallLow: -8,
                overallHigh: 3
            )
            Divider()
        }
    }
    .padding()
}

private let sampleForecasts = [
    DailyForecast(date: "2026-02-15", high: 0, low: -5, symbol: "3", windSpeedAvg: 4.1, precipitationMm: 0),
    DailyForecast(date: "2026-02-16", high: -2, low: -8, symbol: "41", windSpeedAvg: 6.3, precipitationMm: 2.1),
    DailyForecast(date: "2026-02-17", high: 1, low: -3, symbol: "2", windSpeedAvg: 3.0, precipitationMm: 0),
    DailyForecast(date: "2026-02-18", high: 3, low: -1, symbol: "21", windSpeedAvg: 5.5, precipitationMm: 4.8),
    DailyForecast(date: "2026-02-19", high: -1, low: -6, symbol: "1", windSpeedAvg: 2.1, precipitationMm: 0),
]
