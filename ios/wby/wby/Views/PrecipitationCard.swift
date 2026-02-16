import SwiftUI

struct PrecipitationCard: View {
    let forecasts: [DailyForecast]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Label("PRECIPITATION", systemImage: "drop.fill")
                .font(.caption)
                .foregroundStyle(.white.opacity(0.78))

            HStack(alignment: .firstTextBaseline, spacing: 6) {
                Text(todayAmountText)
                    .font(.system(size: 34, weight: .light))
                    .minimumScaleFactor(0.75)
                    .foregroundStyle(.white)
                Text("mm")
                    .font(.system(size: 22, weight: .regular))
                    .foregroundStyle(.white)
            }

            Text(todaySummaryText)
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(.white)

            Spacer(minLength: 0)

            Text(nextMessage)
                .font(.system(size: 14, weight: .regular))
                .foregroundStyle(.white)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(16)
        .frame(height: 220, alignment: .topLeading)
        .frame(maxWidth: .infinity, alignment: .topLeading)
        .background(cardBackground)
    }

    private var todayAmountText: String {
        let mm = precipitationAmount(for: forecasts.first) ?? 0
        if abs(mm.rounded() - mm) < 0.05 {
            return "\(Int(mm.rounded()))"
        }
        return String(format: "%.1f", mm)
    }

    private var todaySummaryText: String {
        guard let pop = forecasts.first?.popAvg else {
            return "Today"
        }
        let percentage = pop <= 1 ? pop * 100 : pop
        return "\(Int(percentage.rounded()))% chance"
    }

    private var nextMessage: String {
        guard let next = forecasts.dropFirst().first(where: { (precipitationAmount(for: $0) ?? 0) >= 0.3 || (precipitationProbability($0) ?? 0) >= 35 }) else {
            return "No significant precipitation expected in the next days."
        }
        let kind = precipitationKind(for: next)
        let typeNote = precipitationTypeText(for: next)
        if let pop = precipitationProbability(next) {
            if let typeNote {
                return "Next \(kind) expected \(displayDate(next.date)) (\(Int(pop.rounded()))% chance, \(typeNote))."
            }
            return "Next \(kind) expected \(displayDate(next.date)) (\(Int(pop.rounded()))% chance)."
        }
        if let typeNote {
            return "Next \(kind) expected \(displayDate(next.date)) (\(typeNote))."
        }
        return "Next \(kind) expected \(displayDate(next.date))."
    }

    private func precipitationAmount(for forecast: DailyForecast?) -> Double? {
        guard let forecast else { return nil }
        return forecast.precipitationMm ?? forecast.precipitation1hSum
    }

    private func precipitationProbability(_ forecast: DailyForecast) -> Double? {
        guard let pop = forecast.popAvg else { return nil }
        return pop <= 1 ? pop * 100 : pop
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

    private func precipitationTypeText(for forecast: DailyForecast) -> String? {
        guard let typeCode = forecast.precipitationTypeMode ?? forecast.potentialPrecipitationTypeMode else {
            return nil
        }
        // FMI/SmartMet code table:
        // 0 = none, 1 = large-scale, 2 = convective
        switch Int(typeCode.rounded()) {
        case 1: return "large-scale"
        case 2: return "convective"
        default: return nil
        }
    }

    private func displayDate(_ raw: String) -> String {
        let parser = DateFormatter()
        parser.dateFormat = "yyyy-MM-dd"
        guard let date = parser.date(from: raw) else { return raw }
        let formatter = DateFormatter()
        formatter.dateFormat = "MMM d"
        return formatter.string(from: date)
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
        PrecipitationCard(
            forecasts: [
                DailyForecast(date: "2026-02-15", high: -5, low: -8, symbol: "3", windSpeedAvg: 3.1, precipitationMm: 0.0),
                DailyForecast(date: "2026-02-16", high: -3, low: -9, symbol: "3", windSpeedAvg: 2.7, precipitationMm: 0.0),
                DailyForecast(date: "2026-02-17", high: -1, low: -6, symbol: "41", windSpeedAvg: 4.2, precipitationMm: 2.4),
            ]
        )
        .padding()
    }
}
