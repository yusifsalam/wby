import SwiftUI

struct DailyNormalsHeadline: View {
    let comparison: DailyNormalsComparison
    var decimals = 0

    private var step: Double { decimals > 0 ? 0.1 : 0.5 }

    var body: some View {
        if let current = comparison.currentTemp, let normal = comparison.normals.today.tempNowNormal, let diff = comparison.nowDiff(step: step) {
            VStack(alignment: .leading, spacing: 2) {
                HStack(alignment: .firstTextBaseline, spacing: 8) {
                    Text("\(DailyNormalsComparison.format(current, decimals: decimals))°")
                        .font(.title)
                        .fontWeight(.semibold)
                    Text("now · normally \(DailyNormalsComparison.format(normal, decimals: decimals))°")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
                Text(text(for: diff))
                    .font(.subheadline)
                    .fontWeight(.medium)
                    .foregroundStyle(DailyNormalsComparison.deltaColor(diff))
            }
        }
    }

    private func text(for diff: Double) -> String {
        if diff > 0 {
            return "\(DailyNormalsComparison.formatNumber(diff))° warmer than usual for this hour"
        } else if diff < 0 {
            return "\(DailyNormalsComparison.formatNumber(-diff))° colder than usual for this hour"
        }
        return "About average for this hour"
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 24) {
        DailyNormalsHeadline(comparison: .preview)
        DailyNormalsHeadline(comparison: .preview, decimals: 1)
    }
    .padding()
}
