import SwiftUI

struct DailyNormalsHeadline: View {
    let comparison: DailyNormalsComparison

    var body: some View {
        if let current = comparison.currentTemp, let normal = comparison.normals.today.tempNowNormal, let diff = comparison.nowDiff(step: 0.1) {
            VStack(alignment: .leading, spacing: 2) {
                HStack(alignment: .firstTextBaseline, spacing: 8) {
                    Text("\(shown(current))°")
                        .font(.title)
                        .fontWeight(.semibold)
                    Text("now · normally \(shown(normal))°")
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

    private func shown(_ value: Double) -> String {
        DailyNormalsComparison.format(value, decimals: 1)
    }

    private func text(for diff: Double) -> String {
        if diff > 0 {
            return "\(DailyNormalsComparison.format(diff, decimals: 1))° warmer than usual for this hour"
        } else if diff < 0 {
            return "\(DailyNormalsComparison.format(-diff, decimals: 1))° colder than usual for this hour"
        }
        return "About average for this hour"
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 24) {
        DailyNormalsHeadline(comparison: .preview)
    }
    .padding()
}
