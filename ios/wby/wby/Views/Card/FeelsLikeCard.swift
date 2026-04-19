import SwiftUI

struct FeelsLikeCard: View {
    let current: CurrentConditions

    var body: some View {
        HalfCard(
            title: "FEELS LIKE",
            icon: "thermometer.medium",
            keyValue: feelsLikeValue,
            description: feelsLikeMessage
        )
    }

    private var feelsLikeValue: String {
        guard let value = current.resolvedFeelsLike ?? current.resolvedTemperature else { return "--" }
        return "\(Int(value.rounded()))°"
    }

    private var feelsLikeMessage: String {
        guard let temp = current.resolvedTemperature, let feelsLike = current.resolvedFeelsLike else {
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
