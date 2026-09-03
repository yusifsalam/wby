import SwiftUI

struct NormalsTemperatureChart: View {
    struct Data {
        struct XLabel: Hashable {
            let index: Int
            let text: String
        }

        let highs: [Double]
        let lows: [Double]
        let avgs: [Double]
        let xLabels: [XLabel]
        let todayIndex: Int
        let currentTemp: Double?
        let todayWeatherHigh: Double?
        let todayWeatherLow: Double?

        var hasBand: Bool { highs.count == avgs.count && lows.count == avgs.count && !avgs.isEmpty }

        var tempMin: Double { (allTemps.min() ?? -10) - 2 }
        var tempMax: Double { (allTemps.max() ?? 30) + 2 }

        private var allTemps: [Double] {
            var all = avgs
            all.append(contentsOf: highs)
            all.append(contentsOf: lows)
            if let currentTemp { all.append(currentTemp) }
            if let todayWeatherHigh { all.append(todayWeatherHigh) }
            if let todayWeatherLow { all.append(todayWeatherLow) }
            return all
        }

        static func dayLabels(count: Int) -> [XLabel] {
            let candidates = [1, 8, 15, 22, count]
            return Array(Set(candidates.filter { $0 >= 1 && $0 <= count })).sorted()
                .map { XLabel(index: $0 - 1, text: "\($0)") }
        }

        static func hourLabels() -> [XLabel] {
            stride(from: 0, to: 24, by: 6).map { XLabel(index: $0, text: String(format: "%02d", $0)) }
        }
    }

    let data: Data

    var body: some View {
        VStack(spacing: 12) {
            temperatureChart
                .frame(height: 120)
            xLabelRow
        }
    }

    // MARK: - Temperature Chart

    private var temperatureChart: some View {
        GeometryReader { geo in
            let width = geo.size.width
            let height = geo.size.height
            let tempRange = data.tempMax - data.tempMin
            let safeRange = tempRange > 0 ? tempRange : 1.0

            ZStack(alignment: .topLeading) {
                yAxisGuidesAndLabels(width: width, height: height, safeRange: safeRange)
                filledBand(width: width, height: height, safeRange: safeRange)
                avgLine(width: width, height: height, safeRange: safeRange)
                todayIndicator(width: width, height: height, safeRange: safeRange)
                currentTempIndicator(width: width, height: height, safeRange: safeRange)
                todayWeatherRangeIndicators(width: width, height: height, safeRange: safeRange)
            }
        }
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

    private func filledBand(width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
        Path { path in
            let count = data.highs.count
            guard data.hasBand else { return }

            for i in 0..<count {
                let x = xPosition(for: i, count: count, width: width)
                let y = yPosition(for: data.highs[i], range: safeRange, min: data.tempMin, height: height)
                if i == 0 {
                    path.move(to: CGPoint(x: x, y: y))
                } else {
                    path.addLine(to: CGPoint(x: x, y: y))
                }
            }

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
    private func yAxisGuidesAndLabels(width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
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

    private func avgLine(width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
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
    private func todayIndicator(width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
        let count = data.avgs.count
        if data.todayIndex >= 0, data.todayIndex < count {
            let x = xPosition(for: data.todayIndex, count: count, width: width)
            let yAvg = yPosition(for: data.avgs[data.todayIndex], range: safeRange, min: data.tempMin, height: height)

            if data.hasBand {
                let yHigh = yPosition(for: data.highs[data.todayIndex], range: safeRange, min: data.tempMin, height: height)
                let yLow = yPosition(for: data.lows[data.todayIndex], range: safeRange, min: data.tempMin, height: height)

                Path { path in
                    path.move(to: CGPoint(x: x, y: yHigh))
                    path.addLine(to: CGPoint(x: x, y: yLow))
                }
                .stroke(.primary.opacity(0.3), lineWidth: 1)
            }

            Circle()
                .fill(.blue)
                .frame(width: 6, height: 6)
                .position(x: x, y: yAvg)
        }
    }

    @ViewBuilder
    private func currentTempIndicator(width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
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
    private func todayWeatherRangeIndicators(width: CGFloat, height: CGFloat, safeRange: Double) -> some View {
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

    // MARK: - X-Axis Labels

    private var xLabelRow: some View {
        GeometryReader { geo in
            let width = geo.size.width
            let count = data.avgs.count

            ForEach(data.xLabels.filter { $0.index >= 0 && $0.index < count }, id: \.self) { label in
                let x = xPosition(for: label.index, count: count, width: width)
                Text(label.text)
                    .font(.system(size: 9))
                    .foregroundStyle(label.index == data.todayIndex ? .primary : .tertiary)
                    .position(x: x, y: 6)
            }
        }
        .frame(height: 12)
    }

    private func yAxisTicks(min: Double, max maxValue: Double) -> [Double] {
        let minTick = floor(min)
        let maxTick = ceil(maxValue)
        let range = maxTick - minTick
        if range <= 0 { return [minTick] }

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
}
