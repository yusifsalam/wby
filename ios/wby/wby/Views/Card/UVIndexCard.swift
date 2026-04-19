import SwiftUI

struct UVIndexCard: View {
    let uvIndex: Double?
    let radiationGlobal: Double?

    // Fallback conversion when only global radiation is available.
    // This keeps card UX consistent (UV index + category) without showing units.
    private var effectiveUVIndex: Double? {
        if let uvIndex {
            return max(0, uvIndex)
        }
        if let radiationGlobal {
            return max(0, radiationGlobal / 100.0)
        }
        return nil
    }

    var body: some View {
        HalfCard(
            title: "UV INDEX",
            icon: "sun.max",
            keyValue: primaryValueText,
            subtitle: secondaryValueText,
            description: category.message
        ) {
            uvBar
        }
    }

    private var uvBar: some View {
        GeometryReader { geo in
            let width = max(geo.size.width, 1)
            let x = normalizedBarPosition * width

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
                    .fill(.primary)
                    .frame(width: 6, height: 6)
                    .offset(x: min(max(x - 3, 0), width - 6))
            }
        }
        .frame(height: 2)
        .padding(.vertical, 6)
    }

    private var primaryValueText: String {
        if let effectiveUVIndex {
            return "\(Int(effectiveUVIndex.rounded()))"
        }
        return "--"
    }

    private var secondaryValueText: String {
        if effectiveUVIndex != nil {
            return category.title
        }
        return "No Data"
    }

    private var normalizedBarPosition: Double {
        if let effectiveUVIndex {
            return min(max(effectiveUVIndex, 0), 11) / 11
        }
        return 0
    }

    private var category: (title: String, message: String) {
        if let effectiveUVIndex {
            switch effectiveUVIndex {
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
        return ("No Data", "No UV or radiation data available.")
    }
}

#Preview {
    ZStack {
        Color.blue.opacity(0.4).ignoresSafeArea()
        UVIndexCard(uvIndex: nil, radiationGlobal: 245)
            .padding()
    }
}
