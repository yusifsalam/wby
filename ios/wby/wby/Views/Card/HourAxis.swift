import SwiftUI

/// "00 06 12 18" under a full-day plot, each label centered on its hour slot.
struct HourAxis: View {
    var body: some View {
        GeometryReader { geo in
            ForEach([0, 6, 12, 18], id: \.self) { hour in
                Text(String(format: "%02d", hour))
                    .font(.caption2)
                    .foregroundStyle(.secondary)
                    .position(x: (CGFloat(hour) + 0.5) / 24 * geo.size.width, y: 7)
            }
        }
        .frame(height: 14)
    }
}

#Preview {
    HourAxis()
        .padding()
}
