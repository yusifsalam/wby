import Foundation

/// The API's `yyyy-MM-dd` calendar days, read and written in a place's time
/// zone independently of the device calendar and locale.
nonisolated enum APIDay {
    static func date(_ string: String, in timeZone: TimeZone) -> Date? {
        try? style(timeZone).parse(string)
    }

    static func string(_ date: Date, in timeZone: TimeZone) -> String {
        style(timeZone).format(date)
    }

    private static func style(_ timeZone: TimeZone) -> Date.ISO8601FormatStyle {
        Date.ISO8601FormatStyle(timeZone: timeZone).year().month().day()
    }
}
