import SwiftUI

struct FeelsLikeCard: View {
    let current: CurrentConditions

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Label("FEELS LIKE", systemImage: "thermometer.medium")
                .font(.caption)
                .foregroundStyle(.white.opacity(0.78))

            Text(feelsLikeValue)
                .font(.system(size: 44, weight: .light))
                .minimumScaleFactor(0.75)
                .foregroundStyle(.white)

            Text(feelsLikeMessage)
                .font(.system(size: 14, weight: .regular))
                .foregroundStyle(.white)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(16)
        .frame(height: 190, alignment: .topLeading)
        .frame(maxWidth: .infinity, alignment: .topLeading)
        .background(cardBackground)
    }

    private var feelsLikeValue: String {
        guard let value = current.feelsLike ?? current.temperature else { return "--" }
        return "\(Int(value.rounded()))°"
    }

    private var feelsLikeMessage: String {
        guard let temp = current.temperature, let feelsLike = current.feelsLike else {
            return "No feels-like data available."
        }
        let delta = feelsLike - temp
        if abs(delta) < 2 {
            return "Similar to the actual temperature."
        }
        if delta < 0 {
            return "Wind can make it feel colder than the actual temperature."
        }
        return "Feels warmer than the actual temperature."
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
        FeelsLikeCard(
            current: CurrentConditions(
                temperature: -9,
                feelsLike: -10,
                windSpeed: 1.0,
                windGust: 2.0,
                windDirection: 266.0,
                humidity: 84.0,
                pressure: 1012.0,
                observedAt: .now
            )
        )
        .padding()
    }
}
