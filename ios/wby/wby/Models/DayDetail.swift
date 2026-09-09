import Foundation

/// One forecast day together with its hours in the place's local calendar
/// day, plus the summary numbers the day sheet shows.
struct DayDetail: Identifiable {
    let day: DailyForecast
    let hours: [HourlyForecast]
    let timeZone: TimeZone

    var id: String { day.date }

    /// Pairs every daily forecast with the hours falling on its local date.
    static func build(days: [DailyForecast], hours: [HourlyForecast], timeZone: TimeZone) -> [DayDetail] {
        let formatter = DateFormatter()
        formatter.dateFormat = "yyyy-MM-dd"
        formatter.timeZone = timeZone
        var byDate: [String: [HourlyForecast]] = [:]
        for hour in hours.sorted(by: { $0.time < $1.time }) {
            byDate[formatter.string(from: hour.time), default: []].append(hour)
        }
        return days.map { DayDetail(day: $0, hours: byDate[$0.date] ?? [], timeZone: timeZone) }
    }

    // MARK: - Dates

    private var calendar: Calendar {
        var calendar = Calendar.current
        calendar.timeZone = timeZone
        return calendar
    }

    var date: Date? { day.displayDate(timeZone: timeZone) }

    var isToday: Bool {
        guard let date else { return false }
        return calendar.isDateInToday(date)
    }

    /// Short name for the toolbar: "Today", "Tomorrow" or the weekday.
    var shortTitle: String {
        guard let date else { return day.date }
        if calendar.isDateInToday(date) { return "Today" }
        if calendar.isDateInTomorrow(date) { return "Tomorrow" }
        return Self.weekdayFormatter(timeZone).string(from: date)
    }

    /// Full date for the sheet header, e.g. "Wednesday 9 September".
    var longTitle: String {
        guard let date else { return day.date }
        return Self.longDateFormatter(timeZone).string(from: date)
    }

    var conditionDescription: String {
        WeatherSymbols.conditionDescription(from: day.symbol) ?? "Forecast"
    }

    /// Hour of day (0–23) of an hourly entry in the place's timezone.
    func localHour(of date: Date) -> Int {
        calendar.component(.hour, from: date)
    }

    /// Where "now" sits in the day as a 0–1 fraction; nil unless this is today.
    var nowFraction: Double? {
        guard isToday else { return nil }
        let now = Date()
        let hour = Double(calendar.component(.hour, from: now))
        let minute = Double(calendar.component(.minute, from: now))
        return (hour + minute / 60) / 24
    }

    // MARK: - Summary

    /// High and low come from the daily row so the sheet repeats the numbers
    /// the user tapped. Precipitation is the total over the hours shown so it
    /// matches the histogram even for today. The rest is derived from hours
    /// where the daily row has nothing.
    var high: Double? { day.high ?? hours.compactMap(\.temperature).max() }
    var low: Double? { day.low ?? hours.compactMap(\.temperature).min() }
    var precipitation: Double? {
        let hourly = hours.compactMap(\.precipitation1h)
        if !hourly.isEmpty { return hourly.reduce(0, +) }
        return day.precipitationMm ?? day.precipitation1hSum
    }
    var popMax: Double? { hours.compactMap(\.pop).max() }
    var windAvg: Double? { day.windSpeedAvg ?? Self.average(hours.compactMap(\.windSpeed)) }
    var gustMax: Double? { day.hourlyMaximumGustMax ?? hours.compactMap(\.windGust).max() }
    var humidityAvg: Double? { day.humidityAvg ?? Self.average(hours.compactMap(\.humidity)) }
    var cloudCoverAvg: Double? { day.totalCloudCoverAvg ?? Self.average(hours.compactMap(\.cloudCover)) }

    struct Stat: Identifiable {
        let label: String
        let value: String

        var id: String { label }
    }

    var stats: [Stat] {
        [
            Stat(label: "High", value: Self.formatTemp(high)),
            Stat(label: "Low", value: Self.formatTemp(low)),
            Stat(label: "Precipitation", value: Self.formatPrecip(precipitation)),
            Stat(label: "Chance of rain", value: Self.formatPercent(popMax)),
            Stat(label: "Wind", value: Self.formatSpeed(windAvg)),
            Stat(label: "Gusts", value: Self.formatSpeed(gustMax)),
            Stat(label: "Humidity", value: Self.formatPercent(humidityAvg)),
            Stat(label: "Cloud cover", value: Self.formatPercent(cloudCoverAvg)),
        ]
    }

    private static func average(_ values: [Double]) -> Double? {
        guard !values.isEmpty else { return nil }
        return values.reduce(0, +) / Double(values.count)
    }

    // MARK: - Formatting

    static func formatTemp(_ value: Double?) -> String {
        guard let value else { return "--" }
        return "\(Int(value.rounded()))°"
    }

    static func formatPrecip(_ value: Double?) -> String {
        guard let value else { return "--" }
        if abs(value.rounded() - value) < 0.05 {
            return "\(Int(value.rounded())) mm"
        }
        return String(format: "%.1f mm", value)
    }

    static func formatPercent(_ value: Double?) -> String {
        guard let value else { return "--" }
        return "\(Int(value.rounded()))%"
    }

    static func formatSpeed(_ value: Double?) -> String {
        guard let value else { return "--" }
        return "\(Int(value.rounded())) m/s"
    }

    private static func weekdayFormatter(_ timeZone: TimeZone) -> DateFormatter {
        let formatter = DateFormatter()
        formatter.dateFormat = "EEEE"
        formatter.timeZone = timeZone
        return formatter
    }

    private static func longDateFormatter(_ timeZone: TimeZone) -> DateFormatter {
        let formatter = DateFormatter()
        formatter.dateFormat = "EEEE d MMMM"
        formatter.timeZone = timeZone
        return formatter
    }
}
