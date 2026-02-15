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

            Text("Today")
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
        let mm = forecasts.first?.precipitationMm ?? 0
        if abs(mm.rounded() - mm) < 0.05 {
            return "\(Int(mm.rounded()))"
        }
        return String(format: "%.1f", mm)
    }

    private var nextMessage: String {
        guard let next = forecasts.dropFirst().first(where: { ($0.precipitationMm ?? 0) >= 0.3 }) else {
            return "No significant precipitation expected in the next days."
        }
        let kind = isSnow(symbol: next.symbol) ? "snow" : "rain"
        return "Next \(kind) expected \(displayDate(next.date))."
    }

    private func displayDate(_ raw: String) -> String {
        let parser = DateFormatter()
        parser.dateFormat = "yyyy-MM-dd"
        guard let date = parser.date(from: raw) else { return raw }
        let formatter = DateFormatter()
        formatter.dateFormat = "MMM d"
        return formatter.string(from: date)
    }

    private func isSnow(symbol: String?) -> Bool {
        guard let symbol else { return false }
        return ["41", "42", "43"].contains(symbol)
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
