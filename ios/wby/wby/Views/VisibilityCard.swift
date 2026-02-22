import SwiftUI

struct VisibilityCard: View {
    let current: CurrentConditions

    var body: some View {
        HalfCard(
            title: "VISIBILITY",
            icon: "eye",
            keyValue: visibilityValue,
            keyValueUnit: visibilityUnit,
            description: visibilityMessage
        )
    }

    private var visibilityMeters: Double? { current.resolvedVisibility }

    private var visibilityValue: String {
        guard let meters = visibilityMeters else { return "--" }
        let km = meters / 1000.0
        if km >= 10 {
            return "\(Int(km.rounded()))"
        } else if km >= 1 {
            return String(format: "%.1f", km)
        } else {
            return "\(Int(meters.rounded()))"
        }
    }

    private var visibilityUnit: String? {
        guard let meters = visibilityMeters else { return nil }
        return meters < 1000 ? "m" : "km"
    }

    private var visibilityMessage: String? {
        guard let meters = visibilityMeters else { return nil }
        switch meters {
        case 30_000...:
            return "Perfectly clear view."
        case 10_000...:
            return "Clear view."
        case 4_000...:
            return "Good visibility."
        case 2_000...:
            return "Moderate visibility."
        case 1_000...:
            return "Poor visibility."
        default:
            return "Very poor visibility."
        }
    }
}

#Preview {
    ZStack {
        Color.blue.opacity(0.4).ignoresSafeArea()
        VisibilityCard(
            current: CurrentConditions(
                temperature: -6,
                feelsLike: -11,
                windSpeed: 3.2,
                windGust: 5.6,
                windDirection: 250,
                humidity: 84,
                pressure: 1012,
                visibility: 37000,
                observedAt: .now
            )
        )
        .padding()
    }
}
