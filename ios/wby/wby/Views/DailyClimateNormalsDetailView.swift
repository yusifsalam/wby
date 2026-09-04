import SwiftUI

struct DailyClimateNormalsDetailView: View {
    let comparison: DailyNormalsComparison

    enum Mode: String, CaseIterable, Identifiable {
        case daily = "Daily"
        case hourly = "Hourly"

        var id: String { rawValue }
    }

    @Environment(\.dismiss) private var dismiss
    @State private var mode: Mode = .daily

    private var chartData: NormalsTemperatureChart.Data {
        mode == .hourly && comparison.hasHourly ? comparison.hourlyChartData : comparison.dailyChartData(withRange: true, withForecastMarkers: true)
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                DailyNormalsHeadline(comparison: comparison, decimals: 1)

                VStack(alignment: .leading, spacing: 12) {
                    HStack {
                        Text(mode == .hourly ? "Typical day, by hour" : "This month, by day")
                            .font(.subheadline)
                            .fontWeight(.medium)
                        Spacer()
                        if comparison.hasHourly {
                            Picker("Normals mode", selection: $mode) {
                                ForEach(Mode.allCases) { mode in
                                    Text(mode.rawValue).tag(mode)
                                }
                            }
                            .pickerStyle(.segmented)
                            .controlSize(.small)
                            .fixedSize()
                        }
                    }
                    NormalsTemperatureChart(data: chartData)
                        .animation(.easeInOut(duration: 0.25), value: mode)
                }
                .weatherCard()

                statGrid

                Text(footnote)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            .padding()
        }
        .navigationTitle("Climate Normals")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .confirmationAction) {
                Button("Done") { dismiss() }
            }
        }
    }

    private var footnote: String {
        let normals = comparison.normals
        var text = "Normals are \(normals.period) averages for \(normals.station.name), \(String(format: "%.1f", normals.station.distanceKm)) km away. Each day is averaged over a window of surrounding days across the period."
        if let gauge = normals.precipitation?.station, gauge.name != normals.station.name {
            text += " Observed precipitation is from the nearest gauge, \(gauge.name), \(String(format: "%.1f", gauge.distanceKm)) km away."
        }
        return text
    }

    private var statRows: [[DailyNormalStat]] {
        let stats = comparison.stats(decimals: 1)
        return stride(from: 0, to: stats.count, by: 2).map { Array(stats[$0..<min($0 + 2, stats.count)]) }
    }

    private var statGrid: some View {
        VStack(spacing: 10) {
            ForEach(statRows, id: \.first?.id) { row in
                HStack(alignment: .top, spacing: 10) {
                    ForEach(row) { stat in
                        statTile(stat)
                    }
                    if row.count == 1 {
                        Color.clear.frame(maxWidth: .infinity)
                    }
                }
                .fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    private func statTile(_ stat: DailyNormalStat) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(stat.label)
                .font(.caption)
                .foregroundStyle(.secondary)
            HStack(alignment: .firstTextBaseline, spacing: 6) {
                Text(stat.value)
                    .font(.title3)
                    .fontWeight(.semibold)
                if let delta = stat.delta {
                    let diff = DailyNormalsComparison.rounded(delta, step: stat.step)
                    Text(DailyNormalsComparison.deltaText(diff, unit: stat.unit))
                        .font(.caption)
                        .fontWeight(.semibold)
                        .foregroundStyle(DailyNormalsComparison.deltaColor(diff))
                }
            }
            Text(stat.detail)
                .font(.caption)
                .foregroundStyle(.secondary)
                .lineLimit(2)
                .minimumScaleFactor(0.8)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .weatherCard()
    }
}

#Preview {
    NavigationStack {
        DailyClimateNormalsDetailView(comparison: .preview)
    }
}
