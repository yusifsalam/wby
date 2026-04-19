import SwiftUI

struct TemperatureLegendView: View {
    private static let labels = ["40", "30", "20", "10", "0", "-20", "-40"]
    private static let colors: [Color] = [
        Color(red: 198.0 / 255.0, green: 29.0 / 255.0, blue: 33.0 / 255.0),   // 40
        Color(red: 235.0 / 255.0, green: 168.0 / 255.0, blue: 58.0 / 255.0),  // 30
        Color(red: 116.0 / 255.0, green: 199.0 / 255.0, blue: 85.0 / 255.0),  // 20
        Color(red: 86.0 / 255.0, green: 208.0 / 255.0, blue: 209.0 / 255.0),  // 10
        Color(red: 96.0 / 255.0, green: 191.0 / 255.0, blue: 255.0 / 255.0),  // 0
        Color(red: 63.0 / 255.0, green: 92.0 / 255.0, blue: 222.0 / 255.0),   // -20
        Color(red: 121.0 / 255.0, green: 45.0 / 255.0, blue: 199.0 / 255.0),  // -40
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
        LinearGradient(
            colors: [Color.blue.opacity(0.65), Color.indigo.opacity(0.85)],
            startPoint: .top,
            endPoint: .bottom
        )
        .ignoresSafeArea()

        TemperatureLegendView()
            .padding()
    }
}
