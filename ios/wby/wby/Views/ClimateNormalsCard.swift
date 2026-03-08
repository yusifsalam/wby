import SwiftUI

struct ClimateNormalsCard: View {
    let normals: ClimateNormalsResponse
    let currentTemp: Double?

    private let monthLabels = ["J", "F", "M", "A", "M", "J", "J", "A", "S", "O", "N", "D"]

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

            // Precipitation bars
            precipitationBars
                .frame(height: 40)

            // Month labels
            monthLabelRow
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
                // Filled band between high and low
                filledBand(data: data, width: width, height: height, safeRange: safeRange)

                // Average temperature line
                avgLine(data: data, width: width, height: height, safeRange: safeRange)

                // Current month highlight
                currentMonthIndicator(data: data, width: width, height: height, safeRange: safeRange)
            }
        }
    }

    private var chartData: ChartData {
        let months = normals.monthly.sorted { $0.month < $1.month }
        let highs = months.map { $0.tempHigh ?? 0 }
        let lows = months.map { $0.tempLow ?? 0 }
        let avgs = months.map { $0.tempAvg ?? 0 }

        let allTemps = highs + lows
        let tempMin = (allTemps.min() ?? -10) - 2
        let tempMax = (allTemps.max() ?? 30) + 2

        return ChartData(highs: highs, lows: lows, avgs: avgs, tempMin: tempMin, tempMax: tempMax)
    }

    private struct ChartData {
        let highs: [Double]
        let lows: [Double]
        let avgs: [Double]
        let tempMin: Double
        let tempMax: Double
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
    private func currentMonthIndicator(data: ChartData, width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
        let currentMonth = Calendar.current.component(.month, from: Date()) - 1
        let count = data.avgs.count
        if currentMonth >= 0, currentMonth < count {
            let x = xPosition(for: currentMonth, count: count, width: width)
            let yAvg = yPosition(for: data.avgs[currentMonth], range: safeRange, min: data.tempMin, height: height)
            let yHigh = yPosition(for: data.highs[currentMonth], range: safeRange, min: data.tempMin, height: height)
            let yLow = yPosition(for: data.lows[currentMonth], range: safeRange, min: data.tempMin, height: height)

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

    // MARK: - Precipitation Bars

    private var precipitationBars: some View {
        GeometryReader { geo in
            let width = geo.size.width
            let height = geo.size.height
            let months = normals.monthly.sorted { $0.month < $1.month }
            let precips = months.map { $0.precipMm ?? 0 }
            let maxPrecip = max(precips.max() ?? 1, 1)
            let count = precips.count

            ForEach(0..<count, id: \.self) { i in
                let barWidth = width / CGFloat(count) * 0.5
                let x = xPosition(for: i, count: count, width: width)
                let barHeight = CGFloat(precips[i] / maxPrecip) * height * 0.85
                let currentMonth = Calendar.current.component(.month, from: Date()) - 1

                RoundedRectangle(cornerRadius: 2)
                    .fill(i == currentMonth ? .cyan.opacity(0.7) : .cyan.opacity(0.35))
                    .frame(width: barWidth, height: max(barHeight, 1))
                    .position(x: x, y: height - barHeight / 2)
            }
        }
    }

    // MARK: - Month Labels

    private var monthLabelRow: some View {
        GeometryReader { geo in
            let width = geo.size.width
            let count = monthLabels.count
            let currentMonth = Calendar.current.component(.month, from: Date()) - 1

            ForEach(0..<count, id: \.self) { i in
                let x = xPosition(for: i, count: count, width: width)
                Text(monthLabels[i])
                    .font(.system(size: 9))
                    .foregroundStyle(i == currentMonth ? .primary : .tertiary)
                    .position(x: x, y: 6)
            }
        }
        .frame(height: 12)
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
            currentTemp: -0.4
        )
        .padding()
    }
}
