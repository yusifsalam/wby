import SwiftUI

struct HumidityCard: View {
    let current: CurrentConditions

    var body: some View {
        HalfCard(
            title: "HUMIDITY",
            icon: "humidity",
            keyValue: humidityValue,
            keyValueUnit: humidityUnit,
            description: dewPointMessage
        )
    }

    private var humidityValue: String {
        guard let humidity = current.resolvedHumidity else { return "--" }
        return "\(Int(humidity.rounded()))"
    }

    private var humidityUnit: String? {
        current.resolvedHumidity != nil ? "%" : nil
    }

    private var dewPointMessage: String? {
        guard let dewPoint = current.resolvedDewPoint else { return nil }
        return "The dew point is \(Int(dewPoint.rounded()))° right now."
    }
}

#Preview {
    ZStack {
        Color.blue.opacity(0.4).ignoresSafeArea()
        HumidityCard(
            current: CurrentConditions(
                temperature: -6,
                feelsLike: -11,
                windSpeed: 3.2,
                windGust: 5.6,
                windDirection: 250,
                humidity: 76,
                dewPoint: -9,
                pressure: 1012,
                observedAt: .now
            )
        )
        .padding()
    }
}
