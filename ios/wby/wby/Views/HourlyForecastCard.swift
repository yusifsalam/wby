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
        .padding()
        .background(cardBackground)
    }

    @ViewBuilder
    private func hourCell(_ hour: HourlyForecast) -> some View {
        VStack(spacing: 8) {
            Text(hourLabel(hour.time))
                .font(.caption.weight(.semibold))
                .foregroundStyle(.white.opacity(0.82))

            Image(systemName: symbolName(for: hour.symbol))
                .frame(height: 20)
                .symbolRenderingMode(.multicolor)

            Text(formatTemp(hour.temperature))
                .font(.headline)
                .foregroundStyle(.white)
        }
        .frame(minWidth: 36)
    }

    private var cardBackground: some View {
        RoundedRectangle(cornerRadius: 14, style: .continuous)
            .fill(.clear)
            .background(.ultraThinMaterial.opacity(0.38), in: RoundedRectangle(cornerRadius: 14, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .stroke(Color.white.opacity(0.2), lineWidth: 1)
            )
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

    private func symbolName(for code: String?) -> String {
        switch code {
        case "1": return "sun.max.fill"
        case "2": return "cloud.sun.fill"
        case "3": return "cloud.fill"
        case "21", "22", "23": return "cloud.rain.fill"
        case "41", "42", "43": return "cloud.snow.fill"
        case "61", "62", "63": return "cloud.bolt.rain.fill"
        default: return "cloud.fill"
        }
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
