import SwiftUI

struct DailyForecastRow: View {
    let forecast: DailyForecast
    let overallLow: Double
    let overallHigh: Double

    var body: some View {
        HStack {
            Text(dayName)
                .frame(width: 44, alignment: .leading)

            Image(systemName: symbolName)
                .frame(width: 30)
                .symbolRenderingMode(.multicolor)

            Text(formatTemp(forecast.low))
                .foregroundStyle(.secondary)
                .frame(width: 36, alignment: .trailing)

            temperatureBar
                .frame(height: 4)

            Text(formatTemp(forecast.high))
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
        switch forecast.symbol {
        case "1": return "sun.max.fill"
        case "2": return "cloud.sun.fill"
        case "3": return "cloud.fill"
        case "21", "22", "23": return "cloud.rain.fill"
        case "41", "42", "43": return "cloud.snow.fill"
        case "61", "62", "63": return "cloud.bolt.rain.fill"
        default: return "cloud.fill"
        }
    }

    @ViewBuilder
    private var temperatureBar: some View {
        GeometryReader { geo in
            let range = overallHigh - overallLow
            let lo = forecast.low ?? overallLow
            let hi = forecast.high ?? overallHigh

            let startFraction = range > 0 ? (lo - overallLow) / range : 0
            let endFraction = range > 0 ? (hi - overallLow) / range : 1

            Capsule()
                .fill(.gray.opacity(0.2))
                .overlay(alignment: .leading) {
                    Capsule()
                        .fill(temperatureGradient)
                        .frame(width: geo.size.width * (endFraction - startFraction))
                        .offset(x: geo.size.width * startFraction)
                }
        }
    }

    private var temperatureGradient: LinearGradient {
        LinearGradient(
            colors: [.blue, .green, .yellow, .orange, .red],
            startPoint: .leading,
            endPoint: .trailing
        )
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
