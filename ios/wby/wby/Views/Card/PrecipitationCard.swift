import SwiftUI

struct PrecipitationCard: View {
    let forecasts: [DailyForecast]
    let timeZone: TimeZone

    var body: some View {
        HalfCard(
            title: "PRECIPITATION",
            icon: "drop.fill",
            keyValue: todayAmountText,
            keyValueUnit: "mm",
            subtitle: "Today",
            description: nextMessage
        )
    }

    private var todayAmountText: String {
        Self.amountText(precipitationAmount(for: forecasts.first) ?? 0)
    }

    private var nextMessage: String {
        let upcoming = forecasts.dropFirst()
        guard let next = upcoming.first(where: { (precipitationAmount(for: $0) ?? 0) >= 0.1 }),
              let mm = precipitationAmount(for: next)
        else {
            return upcoming.isEmpty ? "None expected." : "None expected in the next \(upcoming.count) days."
        }
        return "\(Self.amountText(mm)) mm of \(precipitationKind(for: next)) expected \(dayText(next.date))."
    }

    private static func amountText(_ mm: Double) -> String {
        if abs(mm.rounded() - mm) < 0.05 {
            return "\(Int(mm.rounded()))"
        }
        return String(format: "%.1f", mm)
    }

    private func precipitationAmount(for forecast: DailyForecast?) -> Double? {
        guard let forecast else { return nil }
        return forecast.precipitationMm ?? forecast.precipitation1hSum
    }

    private func precipitationKind(for forecast: DailyForecast) -> String {
        let formCode = forecast.precipitationFormMode ?? forecast.potentialPrecipitationFormMode
        if let formCode {
            // FMI/SmartMet code table:
            // https://github.com/fmidev/smartmet-server/wiki/Weather-data-aggregation-page
            switch Int(formCode.rounded()) {
            case 0: return "drizzle"
            case 1: return "rain"
            case 2: return "sleet"
            case 3: return "snow"
            case 4: return "freezing drizzle"
            case 5: return "freezing rain"
            case 6: return "hail"
            default: break
            }
        }
        return precipitationKindFromSmartSymbol(forecast.symbol)
    }

    private func dayText(_ raw: String) -> String {
        guard let date = APIDay.date(raw, in: timeZone) else { return raw }
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = timeZone
        let days = calendar.dateComponents([.day], from: calendar.startOfDay(for: .now), to: date).day ?? 0
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US")
        formatter.calendar = calendar
        formatter.timeZone = timeZone
        switch days {
        case 1:
            return "tomorrow"
        case 2...6:
            formatter.dateFormat = "EEEE"
            return formatter.string(from: date)
        default:
            formatter.dateFormat = "MMM d"
            return "on \(formatter.string(from: date))"
        }
    }

    private func precipitationKindFromSmartSymbol(_ rawCode: String?) -> String {
        guard let code = SmartSymbol.normalizedCode(from: rawCode) else { return "precipitation" }
        switch code {
        case 11: return "drizzle"
        case 14: return "freezing drizzle"
        case 17: return "freezing rain"
        case 21, 24, 27, 31, 32, 33, 34, 35, 36, 37, 38, 39: return "rain"
        case 41, 42, 43, 44, 45, 46, 47, 48, 49: return "sleet"
        case 51, 52, 53, 54, 55, 56, 57, 58, 59: return "snow"
        case 61, 64, 67: return "hail"
        case 71, 74, 77: return "thundershowers"
        default: return "precipitation"
        }
    }
}

#Preview {
    ZStack {
        Color.blue.opacity(0.4).ignoresSafeArea()
        PrecipitationCard(
            forecasts: [
                DailyForecast(date: "2026-02-15", high: -5, low: -8, symbol: "3", windSpeedAvg: 3.1, precipitationMm: 0.0),
                DailyForecast(date: "2026-02-16", high: -3, low: -9, symbol: "3", windSpeedAvg: 2.7, precipitationMm: 0.0),
                DailyForecast(date: "2026-02-17", high: -1, low: -6, symbol: "41", windSpeedAvg: 4.2, precipitationMm: 2.4),
            ],
            timeZone: TimeZone(identifier: "Europe/Helsinki")!
        )
        .padding()
    }
}
