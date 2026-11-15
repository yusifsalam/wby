import SwiftUI

struct PrecipitationLegendView: View {
    let ramp: MapRampStyle

    private struct Stop {
        let label: String
        let color: Color

        init(_ label: String, _ red: Double, _ green: Double, _ blue: Double) {
            self.label = label
            color = Color(red: red / 255, green: green / 255, blue: blue / 255)
        }
    }

    private static let smoothStops = [
        Stop("50", 255, 0, 170),
        Stop("20", 139, 0, 255),
        Stop("5", 20, 54, 255),
        Stop("1", 63, 160, 255),
        Stop("0.1", 168, 224, 255),
    ]

    private static let steppedClasses = [
        Stop("15", 241, 78, 251),
        Stop("10", 185, 67, 244),
        Stop("5", 130, 56, 236),
        Stop("2.5", 20, 33, 222),
        Stop("1", 57, 85, 229),
        Stop("0.5", 95, 138, 236),
        Stop("0.3", 134, 190, 243),
        Stop("0.1", 173, 243, 250),
    ]

    private static let height: CGFloat = 120

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Precipitation (mm/h)")
                .font(.caption2.bold())
            scale
                .font(.caption2.monospacedDigit())
                .foregroundStyle(.secondary)
        }
        .padding(.vertical, 10)
        .padding(.horizontal, 10)
        .glassEffect(in: RoundedRectangle(cornerRadius: 10))
    }

    @ViewBuilder
    private var scale: some View {
        switch ramp {
        case .smooth:
            HStack(alignment: .top, spacing: 8) {
                LinearGradient(colors: Self.smoothStops.map(\.color), startPoint: .top, endPoint: .bottom)
                    .frame(width: 5, height: Self.height)
                    .clipShape(RoundedRectangle(cornerRadius: 3))
                VStack(alignment: .leading, spacing: 0) {
                    ForEach(Array(Self.smoothStops.enumerated()), id: \.offset) { index, stop in
                        if index > 0 {
                            Spacer(minLength: 0)
                        }
                        Text(stop.label)
                    }
                }
                .frame(height: Self.height)
            }
        case .stepped:
            VStack(alignment: .leading, spacing: 0) {
                ForEach(Self.steppedClasses, id: \.label) { stop in
                    HStack(spacing: 8) {
                        stop.color
                            .frame(width: 5)
                        Text(stop.label)
                    }
                    .frame(height: Self.height / CGFloat(Self.steppedClasses.count))
                }
            }
        }
    }
}

#Preview("Precipitation Legend") {
    HStack(alignment: .top) {
        PrecipitationLegendView(ramp: .smooth)
        PrecipitationLegendView(ramp: .stepped)
    }
}
