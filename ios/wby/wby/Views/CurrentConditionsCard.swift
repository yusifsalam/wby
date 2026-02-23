import SwiftUI

struct CurrentConditionsCard: View {
    let current: CurrentConditions
    private let columns = [GridItem(.flexible()), GridItem(.flexible())]

    var body: some View {
        LazyVGrid(columns: columns, spacing: 12) {
            ForEach(conditionItems, id: \.title) { item in
                conditionItem(title: item.title, value: item.value, icon: item.icon)
            }
        }
        .weatherCard()
    }

    @ViewBuilder
    private func conditionItem(title: String, value: String, icon: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Label(title, systemImage: icon)
                .font(.caption)
                .foregroundStyle(.secondary)
            Text(value)
                .font(.title3)
                .foregroundStyle(.primary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var conditionItems: [(title: String, value: String, icon: String)] {
        [
            ("WIND", formatSpeed(current.resolvedWindSpeed), "wind"),
            ("GUST", formatSpeed(current.resolvedWindGust), "wind"),
            ("HUMIDITY", formatPercent(current.resolvedHumidity), "humidity"),
            ("DEW POINT", formatTemperature(current.resolvedDewPoint), "thermometer.medium"),
            ("PRESSURE", formatPressure(current.resolvedPressure), "gauge.medium"),
            ("PRECIP 1H", formatMillimeters(current.resolvedPrecipitation1h), "drop"),
            ("PRECIP RATE", formatPrecipIntensity(current.resolvedPrecipitationIntensity), "drop.fill"),
            ("SNOW DEPTH", formatSnowDepth(current.resolvedSnowDepth), "snowflake"),
            ("VISIBILITY", formatVisibility(current.resolvedVisibility), "eye"),
            ("CLOUD COVER", formatCloudCover(current.resolvedCloudCover), "cloud"),
            ("WIND DIR", formatWindDir(current.resolvedWindDirection), "location.north.line"),
            ("WEATHER CODE", formatWeatherCode(current.resolvedWeatherCode), "info.circle")
        ]
    }

    private func formatSpeed(_ speed: Double?) -> String {
        guard let speed else { return "--" }
        return String(format: "%.0f m/s", speed)
    }

    private func formatPercent(_ value: Double?) -> String {
        guard let value else { return "--" }
        return "\(Int(value.rounded()))%"
    }

    private func formatTemperature(_ value: Double?) -> String {
        guard let value else { return "--" }
        return "\(Int(value.rounded()))°"
    }

    private func formatPressure(_ p: Double?) -> String {
        guard let p else { return "--" }
        return String(format: "%.0f hPa", p)
    }

    private func formatMillimeters(_ mm: Double?) -> String {
        guard let mm else { return "--" }
        if abs(mm.rounded() - mm) < 0.05 {
            return "\(Int(mm.rounded())) mm"
        }
        return String(format: "%.1f mm", mm)
    }

    private func formatPrecipIntensity(_ value: Double?) -> String {
        guard let value else { return "--" }
        return String(format: "%.1f mm/h", value)
    }

    private func formatSnowDepth(_ value: Double?) -> String {
        guard let value else { return "--" }
        return String(format: "%.0f cm", value)
    }

    private func formatVisibility(_ meters: Double?) -> String {
        guard let meters else { return "--" }
        if meters >= 1000 {
            return String(format: "%.1f km", meters / 1000)
        }
        return String(format: "%.0f m", meters)
    }

    private func formatCloudCover(_ cover: Double?) -> String {
        guard let cover else { return "--" }
        if cover <= 8.5 {
            return "\(Int(cover.rounded()))/8"
        }
        return "\(Int(cover.rounded()))%"
    }

    private func formatWeatherCode(_ code: Double?) -> String {
        guard let code else { return "--" }
        return "\(Int(code.rounded()))"
    }

    private func formatWindDir(_ dir: Double?) -> String {
        guard let dir else { return "--" }
        return "\(Int(dir.rounded()))° \(cardinalDirection(for: dir))"
    }

    private func cardinalDirection(for dir: Double) -> String {
        let directions = ["N", "NE", "E", "SE", "S", "SW", "W", "NW"]
        let index = Int((dir + 22.5) / 45.0) % directions.count
        return directions[index]
    }

}

#Preview {
    CurrentConditionsCard(
        current: CurrentConditions(
            temperature: -4,
            feelsLike: -9,
            windSpeed: 5.2,
            windGust: 7.1,
            windDirection: 220,
            humidity: 89,
            pressure: 1013,
            observedAt: .now
        )
    )
    .padding()
}
