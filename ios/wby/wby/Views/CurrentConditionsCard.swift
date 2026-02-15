import SwiftUI

struct CurrentConditionsCard: View {
    let current: CurrentConditions

    var body: some View {
        LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible())], spacing: 12) {
            conditionItem(title: "WIND", value: formatWind(), icon: "wind")
            conditionItem(title: "HUMIDITY", value: formatPercent(current.humidity), icon: "humidity")
            conditionItem(title: "PRESSURE", value: formatPressure(), icon: "gauge.medium")
            conditionItem(title: "WIND DIR", value: formatWindDir(), icon: "location.north.line")
        }
        .padding()
        .background(cardBackground)
    }

    @ViewBuilder
    private func conditionItem(title: String, value: String, icon: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Label(title, systemImage: icon)
                .font(.caption)
                .foregroundStyle(.white.opacity(0.78))
            Text(value)
                .font(.title3)
                .foregroundStyle(.white)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
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

    private func formatWind() -> String {
        guard let speed = current.windSpeed else { return "--" }
        return String(format: "%.0f m/s", speed)
    }

    private func formatPercent(_ value: Double?) -> String {
        guard let value else { return "--" }
        return "\(Int(value))%"
    }

    private func formatPressure() -> String {
        guard let p = current.pressure else { return "--" }
        return String(format: "%.0f hPa", p)
    }

    private func formatWindDir() -> String {
        guard let dir = current.windDirection else { return "--" }
        let directions = ["N", "NE", "E", "SE", "S", "SW", "W", "NW"]
        let index = Int((dir + 22.5) / 45.0) % 8
        return directions[index]
    }
}

#Preview {
    CurrentConditionsCard(
        current: CurrentConditions(
            temperature: -4,
            feelsLike: -9,
            windSpeed: 5.2,
            windDirection: 220,
            humidity: 89,
            pressure: 1013,
            observedAt: .now
        )
    )
    .padding()
}
