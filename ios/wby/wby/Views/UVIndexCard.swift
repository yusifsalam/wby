import SwiftUI

struct UVIndexCard: View {
    let value: Double

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Label("UV INDEX", systemImage: "sun.max")
                .font(.caption)
                .foregroundStyle(.white.opacity(0.78))

            VStack(alignment: .leading, spacing: 0) {
                Text("\(Int(value.rounded()))")
                    .font(.system(size: 44, weight: .light))
                    .minimumScaleFactor(0.75)
                    .foregroundStyle(.white)
                Text(category.title)
                    .font(.system(size: 20, weight: .semibold))
                    .foregroundStyle(.white)
            }

            uvBar

            Text(category.message)
                .font(.system(size: 14, weight: .regular))
                .foregroundStyle(.white)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(16)
        .frame(height: 190, alignment: .topLeading)
        .frame(maxWidth: .infinity, alignment: .topLeading)
        .background(cardBackground)
    }

    private var uvBar: some View {
        GeometryReader { geo in
            let width = max(geo.size.width, 1)
            let clamped = min(max(value, 0), 11)
            let x = (clamped / 11) * width

            ZStack(alignment: .leading) {
                Capsule()
                    .fill(
                        LinearGradient(
                            colors: [
                                Color(red: 0.44, green: 0.84, blue: 0.36),
                                Color(red: 0.90, green: 0.88, blue: 0.25),
                                Color(red: 0.95, green: 0.58, blue: 0.27),
                                Color(red: 0.95, green: 0.32, blue: 0.34),
                                Color(red: 0.80, green: 0.30, blue: 0.90),
                            ],
                            startPoint: .leading,
                            endPoint: .trailing
                        )
                    )

                Circle()
                    .fill(.white)
                    .frame(width: 6, height: 6)
                    .offset(x: min(max(x - 3, 0), width - 6))
            }
        }
        .frame(height: 2)
        .padding(.vertical, 6)
    }

    private var category: (title: String, message: String) {
        switch value {
        case ..<3:
            return ("Low", "Low for the rest of the day.")
        case ..<6:
            return ("Moderate", "Moderate UV levels expected today.")
        case ..<8:
            return ("High", "High UV levels expected today.")
        case ..<11:
            return ("Very High", "Very high UV levels expected today.")
        default:
            return ("Extreme", "Extreme UV levels expected today.")
        }
    }

    private var cardBackground: some View {
        RoundedRectangle(cornerRadius: 24, style: .continuous)
            .fill(.clear)
            .background(
                .ultraThinMaterial.opacity(0.38),
                in: RoundedRectangle(cornerRadius: 24, style: .continuous)
            )
            .overlay(
                RoundedRectangle(cornerRadius: 24, style: .continuous)
                    .stroke(Color.white.opacity(0.12), lineWidth: 1)
            )
    }
}

#Preview {
    ZStack {
        Color.blue.opacity(0.4).ignoresSafeArea()
        UVIndexCard(value: 0)
            .padding()
    }
}
