import SwiftUI

struct TemperatureLegendView: View {
    private static let labels = ["40", "30", "20", "10", "0", "-20", "-40"]
    private static let colors: [Color] = [
        Color(red: 109.0 / 255.0, green:  22.0 / 255.0, blue:  11.0 / 255.0), //  40 dark red
        Color(red: 210.0 / 255.0, green:  50.0 / 255.0, blue:  40.0 / 255.0), //  30 red
        Color(red: 245.0 / 255.0, green: 210.0 / 255.0, blue:  55.0 / 255.0), //  20 yellow
        Color(red: 180.0 / 255.0, green: 215.0 / 255.0, blue:  75.0 / 255.0), //  10 greenish yellow
        Color(red:  80.0 / 255.0, green: 190.0 / 255.0, blue: 180.0 / 255.0), //   0 greenish blue
        Color(red:  55.0 / 255.0, green: 115.0 / 255.0, blue: 220.0 / 255.0), // -10 blue
        Color(red:  30.0 / 255.0, green:  55.0 / 255.0, blue: 150.0 / 255.0), // -20 dark blue
        Color(red:  80.0 / 255.0, green:  30.0 / 255.0, blue: 130.0 / 255.0), // -40 purple
    ]

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Temperature (°C)")
                .font(.caption2.bold())
            HStack(alignment: .top, spacing: 8) {
                LinearGradient(colors: Self.colors, startPoint: .top, endPoint: .bottom)
                    .frame(width: 5, height: 120)
                    .clipShape(RoundedRectangle(cornerRadius: 3))

                VStack(alignment: .leading, spacing: 5) {
                    ForEach(Self.labels, id: \.self) { label in
                        Text(label)
                    }
                }
                .font(.caption2.monospacedDigit())
                .foregroundStyle(.secondary)
            }
        }
        .padding(.vertical, 10)
        .padding(.horizontal, 10)
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 10))
    }
}

#Preview("Temperature Legend") {
    ZStack {
        TemperatureLegendView()
    }
}
