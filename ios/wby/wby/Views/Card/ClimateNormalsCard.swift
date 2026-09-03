import SwiftUI

struct ClimateNormalsCard: View {
    let normals: ClimateNormalsResponse
    let currentTemp: Double?
    let todayWeatherHigh: Double?
    let todayWeatherLow: Double?

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            // Header
            Label("CLIMATE NORMALS", systemImage: "thermometer.medium")
                .font(.caption)
                .foregroundStyle(.secondary)

            // Period subtitle
            Text(normals.period)
                .font(.caption2)
                .foregroundStyle(.tertiary)

            // Comparison row
            comparisonRow

            NormalsTemperatureChart(data: chartData)
        }
        .weatherCard()
    }

    // MARK: - Comparison Row

    @ViewBuilder
    private var comparisonRow: some View {
        let highPair = todayWeatherHigh.flatMap { high in normals.today.tempHigh.map { (high, $0) } }
        let lowPair = todayWeatherLow.flatMap { low in normals.today.tempLow.map { (low, $0) } }

        if highPair != nil || lowPair != nil {
            VStack(alignment: .leading, spacing: 6) {
                if let (high, normalHigh) = highPair {
                    comparisonLine(label: "High", value: high, normal: normalHigh)
                }
                if let (low, normalLow) = lowPair {
                    comparisonLine(label: "Low", value: low, normal: normalLow)
                }
            }
        } else if let normalTemp = normals.today.tempAvg {
            Text("Normal: \(Int(normalTemp.rounded()))°")
                .font(.subheadline)
                .foregroundStyle(.secondary)
        }
    }

    private func comparisonLine(label: String, value: Double, normal: Double) -> some View {
        HStack(spacing: 8) {
            Text("\(label) \(Int(value.rounded()))°")
                .font(.subheadline)
                .fontWeight(.medium)
            Text("normal \(Int(normal.rounded()))°")
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
        calendar.timeZone = TimeZone(secondsFromGMT: 0) ?? .current

        let now = Date()
        let year = calendar.component(.year, from: now)
        let month = calendar.component(.month, from: now)
        let todayDay = calendar.component(.day, from: now)
        let daysInMonth = calendar.range(of: .day, in: .month, for: now)?.count ?? 30

        let byMonth = Dictionary(uniqueKeysWithValues: normals.monthly.map { ($0.month, $0) })
        let monthlyHighs = (1...12).map { byMonth[$0]?.tempHigh ?? 0 }
        let monthlyLows = (1...12).map { byMonth[$0]?.tempLow ?? 0 }
        let monthlyAvgs = (1...12).map { byMonth[$0]?.tempAvg ?? 0 }

        let highs = (1...daysInMonth).map { day in
            interpolateDaily(monthlyValues: monthlyHighs, year: year, month: month, day: day, calendar: calendar)
        }
        let lows = (1...daysInMonth).map { day in
            interpolateDaily(monthlyValues: monthlyLows, year: year, month: month, day: day, calendar: calendar)
        }
        let avgs = (1...daysInMonth).map { day in
            interpolateDaily(monthlyValues: monthlyAvgs, year: year, month: month, day: day, calendar: calendar)
        }

        let todayIndex = min(max(todayDay - 1, 0), max(daysInMonth - 1, 0))

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

    private func interpolateDaily(
        monthlyValues: [Double],
        year: Int,
        month: Int,
        day: Int,
        calendar: Calendar
    ) -> Double {
        let currentDate = calendar.date(from: DateComponents(year: year, month: month, day: day)) ?? .now
        let midCurrent = calendar.date(from: DateComponents(year: year, month: month, day: 15)) ?? currentDate

        let beforeMonth: Int
        let afterMonth: Int
        let midBefore: Date
        let midAfter: Date

        if day < 15 {
            afterMonth = month
            beforeMonth = month == 1 ? 12 : month - 1
            let beforeYear = month == 1 ? year - 1 : year
            midBefore = calendar.date(from: DateComponents(year: beforeYear, month: beforeMonth, day: 15)) ?? currentDate
            midAfter = midCurrent
        } else {
            beforeMonth = month
            afterMonth = month == 12 ? 1 : month + 1
            let afterYear = month == 12 ? year + 1 : year
            midBefore = midCurrent
            midAfter = calendar.date(from: DateComponents(year: afterYear, month: afterMonth, day: 15)) ?? currentDate
        }

        let totalDuration = midAfter.timeIntervalSince(midBefore)
        guard totalDuration > 0 else { return monthlyValues[month - 1] }
        let elapsed = currentDate.timeIntervalSince(midBefore)
        let t = elapsed / totalDuration
        let weight = (1 - cos(t * .pi)) / 2

        let beforeValue = monthlyValues[beforeMonth - 1]
        let afterValue = monthlyValues[afterMonth - 1]
        return beforeValue * (1 - weight) + afterValue * weight
    }
}

// MARK: - Preview

#Preview {
    ZStack {
        Color.blue.opacity(0.4).ignoresSafeArea()
        ClimateNormalsCard(
            normals: ClimateNormalsResponse(
                station: StationInfo(name: "Helsinki Kaisaniemi", distanceKm: 1.2),
                period: "1991-2020",
                today: InterpolatedNormal(
                    tempAvg: -4.2,
                    tempHigh: -1.5,
                    tempLow: -8.1,
                    precipMmDay: 1.4,
                    tempDiff: 3.8
                ),
                monthly: (1...12).map { month in
                    MonthlyNormal(
                        month: month,
                        tempAvg: [-6.2, -6.8, -2.8, 3.8, 10.2, 14.8, 17.6, 16.2, 11.0, 5.8, 0.4, -4.0][month - 1],
                        tempHigh: [-3.1, -3.2, 1.2, 8.4, 15.2, 19.4, 22.4, 20.8, 15.0, 8.8, 3.0, -1.0][month - 1],
                        tempLow: [-9.4, -10.4, -6.8, -0.8, 4.8, 10.0, 13.2, 12.0, 7.4, 2.8, -2.2, -7.0][month - 1],
                        precipMm: [52, 36, 34, 32, 37, 57, 63, 80, 56, 76, 68, 58][month - 1]
                    )
                }
            ),
            currentTemp: -0.4,
            todayWeatherHigh: 1.5,
            todayWeatherLow: -6.0
        )
        .padding()
    }
}
