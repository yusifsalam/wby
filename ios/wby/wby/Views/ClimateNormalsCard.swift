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

            // Temperature chart
            temperatureChart
                .frame(height: 120)

            // Day labels for current month
            dayLabelRow
        }
        .weatherCard()
    }

    // MARK: - Comparison Row

    @ViewBuilder
    private var comparisonRow: some View {
        if let normalTemp = normals.today.tempAvg {
            HStack(spacing: 8) {
                Text("Normal: \(Int(normalTemp.rounded()))°")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)

                if let diff = normals.today.tempDiff {
                    diffBadge(diff)
                } else if let current = currentTemp {
                    diffBadge(current - normalTemp)
                }
            }
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

    // MARK: - Temperature Chart

    private var temperatureChart: some View {
        GeometryReader { geo in
            let width = geo.size.width
            let height = geo.size.height
            let data = chartData
            let tempRange = data.tempMax - data.tempMin
            let safeRange = tempRange > 0 ? tempRange : 1.0

            ZStack(alignment: .topLeading) {
                // Horizontal y-axis guides and labels for all tick steps.
                yAxisGuidesAndLabels(data: data, width: width, height: height, safeRange: safeRange)

                // Filled band between high and low
                filledBand(data: data, width: width, height: height, safeRange: safeRange)

                // Average temperature line
                avgLine(data: data, width: width, height: height, safeRange: safeRange)

                // Today's marker
                todayIndicator(data: data, width: width, height: height, safeRange: safeRange)

                // Actual current temperature marker
                currentTempIndicator(data: data, width: width, height: height, safeRange: safeRange)

                // Today's weather high/low markers
                todayWeatherRangeIndicators(data: data, width: width, height: height, safeRange: safeRange)
            }
        }
    }

    private var chartData: ChartData {
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

        var allTemps = highs
        allTemps.append(contentsOf: lows)
        if let currentTemp {
            allTemps.append(currentTemp)
        }
        if let todayWeatherHigh {
            allTemps.append(todayWeatherHigh)
        }
        if let todayWeatherLow {
            allTemps.append(todayWeatherLow)
        }
        let tempMin = (allTemps.min() ?? -10) - 2
        let tempMax = (allTemps.max() ?? 30) + 2
        let todayIndex = min(max(todayDay - 1, 0), max(daysInMonth - 1, 0))

        return ChartData(
            highs: highs,
            lows: lows,
            avgs: avgs,
            tempMin: tempMin,
            tempMax: tempMax,
            todayIndex: todayIndex,
            currentTemp: currentTemp,
            todayWeatherHigh: todayWeatherHigh,
            todayWeatherLow: todayWeatherLow
        )
    }

    private struct ChartData {
        let highs: [Double]
        let lows: [Double]
        let avgs: [Double]
        let tempMin: Double
        let tempMax: Double
        let todayIndex: Int
        let currentTemp: Double?
        let todayWeatherHigh: Double?
        let todayWeatherLow: Double?
    }

    private func xPosition(for index: Int, count: Int, width: CGFloat) -> CGFloat {
        guard count > 1 else { return width / 2 }
        let spacing = width / CGFloat(count)
        return spacing * CGFloat(index) + spacing / 2
    }

    private func yPosition(for temp: Double, range: Double, min: Double, height: CGFloat) -> CGFloat {
        let normalized = (temp - min) / range
        return height * (1 - normalized)
    }

    private func filledBand(data: ChartData, width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
        Path { path in
            let count = data.highs.count
            guard count > 0 else { return }

            // Top edge (highs) left to right
            for i in 0..<count {
                let x = xPosition(for: i, count: count, width: width)
                let y = yPosition(for: data.highs[i], range: safeRange, min: data.tempMin, height: height)
                if i == 0 {
                    path.move(to: CGPoint(x: x, y: y))
                } else {
                    path.addLine(to: CGPoint(x: x, y: y))
                }
            }

            // Bottom edge (lows) right to left
            for i in stride(from: count - 1, through: 0, by: -1) {
                let x = xPosition(for: i, count: count, width: width)
                let y = yPosition(for: data.lows[i], range: safeRange, min: data.tempMin, height: height)
                path.addLine(to: CGPoint(x: x, y: y))
            }

            path.closeSubpath()
        }
        .fill(.blue.opacity(0.15))
    }

    @ViewBuilder
    private func yAxisGuidesAndLabels(data: ChartData, width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
        let ticks = yAxisTicks(min: data.tempMin, max: data.tempMax)
        ForEach(ticks, id: \.self) { tick in
            let y = yPosition(for: tick, range: safeRange, min: data.tempMin, height: height)
            let clampedY = min(max(y, 8), max(height - 8, 8))

            Path { path in
                path.move(to: CGPoint(x: 0, y: clampedY))
                path.addLine(to: CGPoint(x: width, y: clampedY))
            }
            .stroke(.primary.opacity(0.08), lineWidth: 1)

            Text("\(Int(tick.rounded()))°")
                .font(.caption2)
                .foregroundStyle(.tertiary)
                .position(x: width - 14, y: clampedY)
        }
    }

    private func avgLine(data: ChartData, width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
        Path { path in
            let count = data.avgs.count
            guard count > 0 else { return }

            for i in 0..<count {
                let x = xPosition(for: i, count: count, width: width)
                let y = yPosition(for: data.avgs[i], range: safeRange, min: data.tempMin, height: height)
                if i == 0 {
                    path.move(to: CGPoint(x: x, y: y))
                } else {
                    path.addLine(to: CGPoint(x: x, y: y))
                }
            }
        }
        .stroke(.blue.opacity(0.7), lineWidth: 2)
    }

    @ViewBuilder
    private func todayIndicator(data: ChartData, width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
        let count = data.avgs.count
        if data.todayIndex >= 0, data.todayIndex < count {
            let x = xPosition(for: data.todayIndex, count: count, width: width)
            let yAvg = yPosition(for: data.avgs[data.todayIndex], range: safeRange, min: data.tempMin, height: height)
            let yHigh = yPosition(for: data.highs[data.todayIndex], range: safeRange, min: data.tempMin, height: height)
            let yLow = yPosition(for: data.lows[data.todayIndex], range: safeRange, min: data.tempMin, height: height)

            // Vertical line through the band
            Path { path in
                path.move(to: CGPoint(x: x, y: yHigh))
                path.addLine(to: CGPoint(x: x, y: yLow))
            }
            .stroke(.primary.opacity(0.3), lineWidth: 1)

            // Dot on the average
            Circle()
                .fill(.blue)
                .frame(width: 6, height: 6)
                .position(x: x, y: yAvg)
        }
    }

    @ViewBuilder
    private func currentTempIndicator(data: ChartData, width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
        if let currentTemp = data.currentTemp {
            let count = data.avgs.count
            if data.todayIndex >= 0, data.todayIndex < count {
                let x = xPosition(for: data.todayIndex, count: count, width: width)
                let yNow = yPosition(for: currentTemp, range: safeRange, min: data.tempMin, height: height)
                let clampedY = min(max(yNow, 6), max(height - 6, 6))
                let wantsLeft = x < width * 0.65
                let proposedLabelX = wantsLeft ? x - 30 : x + 30
                let clampedLabelX = min(max(proposedLabelX, 20), width - 20)
                let labelY = max(clampedY - 10, 10)

                Circle()
                    .fill(.orange)
                    .frame(width: 7, height: 7)
                    .position(x: x, y: clampedY)

                Text("Now \(Int(currentTemp.rounded()))°")
                    .font(.caption2)
                    .fontWeight(.semibold)
                    .foregroundStyle(.orange.opacity(0.95))
                    .position(x: clampedLabelX, y: labelY)
            }
        }
    }

    @ViewBuilder
    private func todayWeatherRangeIndicators(data: ChartData, width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
        let count = data.avgs.count
        if data.todayIndex >= 0, data.todayIndex < count,
           let high = data.todayWeatherHigh,
           let low = data.todayWeatherLow
        {
            let x = xPosition(for: data.todayIndex, count: count, width: width)
            let yHigh = yPosition(for: high, range: safeRange, min: data.tempMin, height: height)
            let yLow = yPosition(for: low, range: safeRange, min: data.tempMin, height: height)

            let labelX = x < width * 0.65 ? x + 22 : x - 22
            let clampedLabelX = min(max(labelX, 18), width - 18)
            let highLabelY = max(yHigh - 10, 10)
            let lowLabelY = min(yLow + 10, height - 10)

            Image(systemName: "triangle.fill")
                .font(.system(size: 8, weight: .bold))
                .foregroundStyle(.red.opacity(0.9))
                .position(x: x, y: yHigh)

            Image(systemName: "triangle.fill")
                .font(.system(size: 8, weight: .bold))
                .rotationEffect(.degrees(180))
                .foregroundStyle(.indigo.opacity(0.9))
                .position(x: x, y: yLow)

            Text("H \(Int(high.rounded()))°")
                .font(.caption2)
                .fontWeight(.semibold)
                .foregroundStyle(.red.opacity(0.9))
                .position(x: clampedLabelX, y: highLabelY)

            Text("L \(Int(low.rounded()))°")
                .font(.caption2)
                .fontWeight(.semibold)
                .foregroundStyle(.indigo.opacity(0.9))
                .position(x: clampedLabelX, y: lowLabelY)
        }
    }

    // MARK: - Day Labels

    private var dayLabelRow: some View {
        GeometryReader { geo in
            let width = geo.size.width
            let data = chartData
            let count = data.avgs.count
            let ticks = dayTicks(for: count)

            ForEach(ticks, id: \.self) { day in
                let index = day - 1
                let x = xPosition(for: index, count: count, width: width)
                Text("\(day)")
                    .font(.system(size: 9))
                    .foregroundStyle(index == data.todayIndex ? .primary : .tertiary)
                    .position(x: x, y: 6)
            }
        }
        .frame(height: 12)
    }

    private func dayTicks(for dayCount: Int) -> [Int] {
        let candidates = [1, 8, 15, 22, dayCount]
        return Array(Set(candidates.filter { $0 >= 1 && $0 <= dayCount })).sorted()
    }

    private func yAxisTicks(min: Double, max maxValue: Double) -> [Double] {
        let minTick = floor(min)
        let maxTick = ceil(maxValue)
        let range = maxTick - minTick
        if range <= 0 { return [minTick] }

        // Keep labels readable in the compact card while still showing full scale steps.
        let targetTickCount = 6.0
        let rawStep = range / Swift.max(targetTickCount - 1, 1)
        let step = niceStep(rawStep)

        let start = ceil(minTick / step) * step
        let end = floor(maxTick / step) * step

        var ticks: [Double] = []
        if start <= end {
            var current = start
            while current <= end + step * 0.5 {
                ticks.append(current)
                current += step
            }
        } else {
            ticks = [minTick, maxTick]
        }
        return ticks
    }

    private func niceStep(_ value: Double) -> Double {
        let exponent = floor(log10(value))
        let fraction = value / pow(10, exponent)
        let niceFraction: Double
        if fraction <= 1 {
            niceFraction = 1
        } else if fraction <= 2 {
            niceFraction = 2
        } else if fraction <= 5 {
            niceFraction = 5
        } else {
            niceFraction = 10
        }
        return niceFraction * pow(10, exponent)
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
            todayWeatherHigh: 2.0,
            todayWeatherLow: -9.0
        )
        .padding()
    }
}
