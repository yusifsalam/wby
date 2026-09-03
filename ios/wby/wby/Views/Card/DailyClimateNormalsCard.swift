import SwiftUI

struct DailyClimateNormalsCard: View {
    let normals: DailyClimateNormalsResponse
    let currentTemp: Double?
    let todayWeatherHigh: Double?
    let todayWeatherLow: Double?
    let timeZone: TimeZone

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Label("CLIMATE NORMALS · DAILY", systemImage: "thermometer.medium")
                .font(.caption)
                .foregroundStyle(.secondary)

            Text("\(normals.station.name) · \(normals.period)")
                .font(.caption2)
                .foregroundStyle(.tertiary)

            comparisonRows

            NormalsTemperatureChart(data: chartData)
        }
        .weatherCard()
    }

    // MARK: - Comparison Rows

    @ViewBuilder
    private var comparisonRows: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let current = currentTemp, let nowNormal = normals.today.tempNowNormal {
                comparisonLine(label: "Now", value: current, normal: nowNormal, normalLabel: "normal at this hour")
            }
            if let high = todayWeatherHigh, let normalHigh = normals.today.tempHigh {
                comparisonLine(label: "High", value: high, normal: normalHigh, normalLabel: "normal")
            }
            if let low = todayWeatherLow, let normalLow = normals.today.tempLow {
                comparisonLine(label: "Low", value: low, normal: normalLow, normalLabel: "normal")
            }
        }
    }

    private func comparisonLine(label: String, value: Double, normal: Double, normalLabel: String) -> some View {
        HStack(spacing: 8) {
            Text("\(label) \(Int(value.rounded()))°")
                .font(.subheadline)
                .fontWeight(.medium)
            Text("\(normalLabel) \(Int(normal.rounded()))°")
                .font(.subheadline)
                .foregroundStyle(.secondary)
            diffBadge(value - normal)
        }
    }

    private func diffBadge(_ diff: Double) -> some View {
        let rounded = (diff * 10).rounded() / 10
        let text: String
        let color: Color
        if rounded > 0 {
            text = "+\(formatDiff(rounded))° warmer"
            color = .red
        } else if rounded < 0 {
            text = "\(formatDiff(rounded))° colder"
            color = .blue
        } else {
            text = "0° average"
            color = .secondary
        }
        return Text(text)
            .font(.caption)
            .fontWeight(.medium)
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .background(color.opacity(0.18), in: Capsule())
            .foregroundStyle(color)
    }

    private func formatDiff(_ value: Double) -> String {
        if value == value.rounded() {
            return "\(Int(value))"
        }
        return String(format: "%.1f", value)
    }

    // MARK: - Chart Data

    private var chartData: NormalsTemperatureChart.Data {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = timeZone

        let now = Date()
        let month = calendar.component(.month, from: now)
        let todayDay = calendar.component(.day, from: now)

        let days = normals.daily
            .filter { $0.month == month }
            .sorted { $0.day < $1.day }
        let highs = days.map { $0.tempHigh ?? $0.tempAvg ?? 0 }
        let lows = days.map { $0.tempLow ?? $0.tempAvg ?? 0 }
        let avgs = days.map { $0.tempAvg ?? 0 }
        let todayIndex = min(max(todayDay - 1, 0), max(days.count - 1, 0))

        return NormalsTemperatureChart.Data(
            highs: highs,
            lows: lows,
            avgs: avgs,
            todayIndex: todayIndex,
            currentTemp: currentTemp,
            todayWeatherHigh: todayWeatherHigh,
            todayWeatherLow: todayWeatherLow
        )
    }
}

// MARK: - Preview

#Preview {
    let daily: [DailyNormal] = (1...12).flatMap { month in
        (1...31).map { day in
            let doy = Double((month - 1) * 30 + day)
            let avg = 5 - 12 * cos(2 * .pi * (doy - 15) / 365)
            return DailyNormal(month: month, day: day, tempAvg: avg, tempHigh: avg + 3.5, tempLow: avg - 3.2, precipMm: 2)
        }
    }
    let hourly = (0..<24).map { hour in 14.3 + 2.4 * sin(2 * .pi * Double(hour - 9) / 24) }
    ZStack {
        Color.blue.opacity(0.4).ignoresSafeArea()
        DailyClimateNormalsCard(
            normals: DailyClimateNormalsResponse(
                station: StationInfo(name: "Helsinki Kaisaniemi", distanceKm: 0.6),
                period: "1991-2020",
                today: DailyNormalToday(
                    month: 9, day: 3, tempAvg: 14.3, tempHigh: 17.7, tempLow: 11.3, precipMm: 2.3,
                    tempHourly: hourly, tempNowNormal: 13.9, tempDiff: 0.1
                ),
                daily: daily
            ),
            currentTemp: 14,
            todayWeatherHigh: 18.7,
            todayWeatherLow: 13.9,
            timeZone: TimeZone(identifier: "Europe/Helsinki") ?? .current
        )
        .padding()
    }
}
