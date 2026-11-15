import SwiftUI

/// Hourly chance of precipitation for one local day on a fixed 0–100% axis.
struct PrecipitationChanceChart: View {
    let detail: DayDetail

    private static let axisWidth: CGFloat = 36
    private static let plotHeight: CGFloat = 120
    private static let ticks = stride(from: 0.0, through: 100.0, by: 20.0).map { $0 }

    var body: some View {
        VStack(spacing: 6) {
            HStack(spacing: 0) {
                plot
                    .frame(height: Self.plotHeight)
                axisLabels
                    .frame(width: Self.axisWidth, height: Self.plotHeight)
            }
            HStack(spacing: 0) {
                HourAxis()
                Color.clear.frame(width: Self.axisWidth, height: 1)
            }
        }
    }

    private var plot: some View {
        GeometryReader { geo in
            let width = geo.size.width
            let height = geo.size.height
            let barWidth = width / 24 * 0.7
            let now = Date()
            ZStack(alignment: .topLeading) {
                Path { path in
                    for tick in Self.ticks {
                        let y = height * (1 - tick / 100)
                        path.move(to: CGPoint(x: 0, y: y))
                        path.addLine(to: CGPoint(x: width, y: y))
                    }
                }
                .stroke(.primary.opacity(0.08), lineWidth: 1)
                ForEach(detail.hours) { hour in
                    if let pop = hour.pop, pop > 0 {
                        let barHeight = max(height * min(pop, 100) / 100, 2)
                        RoundedRectangle(cornerRadius: 2, style: .continuous)
                            .fill(Color.cyan)
                            .frame(width: barWidth, height: barHeight)
                            .position(
                                x: (CGFloat(detail.localHour(of: hour.time)) + 0.5) / 24 * width,
                                y: height - barHeight / 2
                            )
                            .opacity(hour.time.addingTimeInterval(3600) <= now ? 0.35 : 1)
                    }
                }
                if let fraction = detail.nowFraction {
                    Rectangle()
                        .fill(.primary.opacity(0.3))
                        .frame(width: 1, height: height)
                        .position(x: CGFloat(fraction) * width, y: height / 2)
                }
            }
        }
    }

    private var axisLabels: some View {
        GeometryReader { geo in
            ForEach(Self.ticks, id: \.self) { tick in
                Text("\(Int(tick))%")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .position(x: geo.size.width / 2 + 4, y: min(max(geo.size.height * (1 - tick / 100), 6), geo.size.height - 6))
            }
        }
    }
}

#Preview {
    let detail = DayDetail.build(
        days: [PreviewData.makeDaily()[0]],
        hours: PreviewData.makeFullDay(),
        timeZone: TimeZone(identifier: "Europe/Helsinki")!
    )[0]
    PrecipitationChanceChart(detail: detail)
        .weatherCard()
        .padding()
}
