import SwiftUI

struct DailyClimateNormalsCard: View {
    let normals: DailyClimateNormalsResponse
    let currentTemp: Double?
    let currentFeelsLike: Double?
    let currentWind: Double?
    let currentHumidity: Double?
    let currentSnowDepth: Double?
    let todayForecast: DailyForecast?
    let timeZone: TimeZone

    @State private var showingDetail = false

    private var comparison: DailyNormalsComparison {
        DailyNormalsComparison(
            normals: normals,
            currentTemp: currentTemp,
            currentFeelsLike: currentFeelsLike,
            currentWind: currentWind,
            currentHumidity: currentHumidity,
            currentSnowDepth: currentSnowDepth,
            todayForecast: todayForecast,
            timeZone: timeZone
        )
    }

    var body: some View {
        let comparison = comparison
        Button {
            showingDetail = true
        } label: {
            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    Label("CLIMATE NORMALS", systemImage: "thermometer.medium")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Spacer()
                    Image(systemName: "chevron.right")
                        .font(.caption)
                        .foregroundStyle(.tertiary)
                }

                DailyNormalsHeadline(comparison: comparison)

                NormalsTemperatureChart(data: comparison.hasHourly ? comparison.hourlyChartData : comparison.dailyChartData(withRange: true, withForecastMarkers: false))

                summaryRow(comparison.summaryStats(decimals: 0))

                Text("\(normals.station.name) · \(normals.period)")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
            }
            .weatherCard()
        }
        .buttonStyle(.plain)
        .sheet(isPresented: $showingDetail) {
            NavigationStack {
                DailyClimateNormalsDetailView(comparison: comparison)
            }
        }
    }

    @ViewBuilder
    private func summaryRow(_ stats: [DailyNormalStat]) -> some View {
        if !stats.isEmpty {
            HStack(alignment: .top, spacing: 0) {
                ForEach(stats) { stat in
                    VStack(alignment: .leading, spacing: 2) {
                        Text(stat.label)
                            .font(.caption2)
                            .foregroundStyle(.tertiary)
                        HStack(alignment: .firstTextBaseline, spacing: 4) {
                            Text(stat.value)
                                .font(.subheadline)
                                .fontWeight(.semibold)
                            if let delta = stat.delta {
                                let diff = DailyNormalsComparison.rounded(delta, step: stat.step)
                                Text(DailyNormalsComparison.deltaText(diff, unit: stat.unit))
                                    .font(.caption2)
                                    .fontWeight(.semibold)
                                    .foregroundStyle(DailyNormalsComparison.deltaColor(diff))
                            }
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
    }
}

// MARK: - Preview

#Preview {
    let comparison = DailyNormalsComparison.preview
    ZStack {
        Color.blue.opacity(0.4).ignoresSafeArea()
        DailyClimateNormalsCard(
            normals: comparison.normals,
            currentTemp: comparison.currentTemp,
            currentFeelsLike: comparison.currentFeelsLike,
            currentWind: comparison.currentWind,
            currentHumidity: comparison.currentHumidity,
            currentSnowDepth: comparison.currentSnowDepth,
            todayForecast: comparison.todayForecast,
            timeZone: comparison.timeZone
        )
        .padding()
    }
}
