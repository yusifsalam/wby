import SwiftUI

struct HourlyForecastCard: View {
    let hourly: [HourlyForecast]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Label("HOURLY FORECAST", systemImage: "clock")
                .font(.caption)
                .foregroundStyle(.white.opacity(0.8))

            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: 18) {
                    ForEach(hourly) { hour in
                        hourCell(hour)
                    }
                }
            }
        }
        .weatherCard()
    }

    @ViewBuilder
    private func hourCell(_ hour: HourlyForecast) -> some View {
        VStack(spacing: 8) {
            Text(hourLabel(hour.time))
                .font(.caption.weight(.semibold))
                .foregroundStyle(.white.opacity(0.82))

            Image(systemName: SmartSymbol.systemImageName(for: hour.symbol))
                .frame(height: 20)
                .symbolRenderingMode(.multicolor)

            Text(formatTemp(hour.temperature))
                .font(.headline)
                .foregroundStyle(.white)

            Text(formatWind(hour.windSpeed))
                .font(.caption2)
                .foregroundStyle(.white.opacity(0.72))

            Text(formatPrecip(hour.precipitation1h))
                .font(.caption2)
                .foregroundStyle(.white.opacity(0.72))

            Text(formatHumidity(hour.humidity))
                .font(.caption2)
                .foregroundStyle(.white.opacity(0.72))
        }
        .frame(minWidth: 48)
    }

    private func hourLabel(_ date: Date) -> String {
        if Calendar.current.isDate(date, equalTo: Date(), toGranularity: .hour) {
            return "Now"
        }
        let formatter = DateFormatter()
        formatter.dateFormat = "HH"
        return formatter.string(from: date)
    }

    private func formatTemp(_ temp: Double?) -> String {
        guard let temp else { return "--" }
        return "\(Int(temp.rounded()))°"
    }

    private func formatWind(_ speed: Double?) -> String {
        guard let speed else { return "-- m/s" }
        return "\(Int(speed.rounded())) m/s"
    }

    private func formatPrecip(_ mm: Double?) -> String {
        guard let mm else { return "-- mm" }
        if abs(mm.rounded() - mm) < 0.05 {
            return "\(Int(mm.rounded())) mm"
        }
        return String(format: "%.1f mm", mm)
    }

    private func formatHumidity(_ value: Double?) -> String {
        guard let value else { return "--%" }
        return "\(Int(value.rounded()))%"
    }

}

#Preview {
    HourlyForecastCard(hourly: [
        HourlyForecast(time: .now, temperature: -11, symbol: "2"),
        HourlyForecast(time: .now.addingTimeInterval(3600), temperature: -11, symbol: "2"),
        HourlyForecast(time: .now.addingTimeInterval(7200), temperature: -10, symbol: "3"),
        HourlyForecast(time: .now.addingTimeInterval(10800), temperature: -10, symbol: "3"),
        HourlyForecast(time: .now.addingTimeInterval(14400), temperature: -9, symbol: "3"),
        HourlyForecast(time: .now.addingTimeInterval(18000), temperature: -9, symbol: "3"),
    ])
    .padding()
}
