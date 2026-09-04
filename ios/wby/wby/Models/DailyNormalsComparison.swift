import Foundation
import SwiftUI

struct DailyNormalStat: Identifiable {
    let label: String
    let value: String
    var detail: String
    let delta: Double?
    let unit: String
    let step: Double

    var id: String { label }
}

struct DailyNormalsComparison {
    let normals: DailyClimateNormalsResponse
    let currentTemp: Double?
    let currentFeelsLike: Double?
    let currentWind: Double?
    let currentHumidity: Double?
    let currentSnowDepth: Double?
    let todayForecast: DailyForecast?
    let timeZone: TimeZone

    var hasHourly: Bool { normals.today.tempHourly?.count == 24 }

    var todayWeatherHigh: Double? { todayForecast?.high }
    var todayWeatherLow: Double? { todayForecast?.low }

    /// Wind chill from air temperature (°C) and wind (m/s), matching the
    /// server's feels-like for observations and the normals' hourly samples.
    static func feelsLike(temp: Double?, wind: Double?) -> Double? {
        guard let temp else { return nil }
        guard let wind else { return temp }
        let windKmh = wind * 3.6
        if temp > 10 || windKmh < 4.8 {
            return temp
        }
        return 13.12 + 0.6215 * temp - 11.37 * pow(windKmh, 0.16) + 0.3965 * temp * pow(windKmh, 0.16)
    }

    func nowDiff(step: Double) -> Double? {
        guard let current = currentTemp, let normal = normals.today.tempNowNormal else { return nil }
        return Self.rounded(current - normal, step: step)
    }

    // MARK: - Stats

    /// Detail grid, in row pairs: mean/feels-like mean, high/low, feels-like
    /// high/low, wind/gusts, humidity/precipitation, then snow when present.
    /// `decimals` controls both value formatting and delta rounding: 0 shows
    /// whole numbers with deltas on a half-degree/whole-unit step, 1 shows
    /// tenths everywhere.
    func stats(decimals: Int) -> [DailyNormalStat] {
        let today = normals.today
        let forecast = todayForecast
        let tempStep = decimals > 0 ? 0.1 : 0.5
        let unitStep = Self.unitStep(decimals)
        var stats: [DailyNormalStat] = []
        if let stat = Self.stat(label: "MEAN", current: forecast?.temperatureAvg, normal: today.tempAvg, unit: "°", step: tempStep, decimals: decimals) {
            stats.append(stat)
        }
        if let stat = Self.stat(label: "FEELS LIKE MEAN", current: Self.feelsLike(temp: forecast?.temperatureAvg, wind: forecast?.windSpeedAvg), normal: today.feelsLikeAvg, unit: "°", step: tempStep, decimals: decimals) {
            stats.append(stat)
        }
        if let stat = highStat(decimals: decimals) {
            stats.append(stat)
        }
        if let stat = lowStat(decimals: decimals) {
            stats.append(stat)
        }
        if let stat = Self.stat(label: "FEELS LIKE HIGH", current: Self.feelsLike(temp: forecast?.high, wind: forecast?.windSpeedAvg), normal: today.feelsLikeHigh, unit: "°", step: tempStep, decimals: decimals) {
            stats.append(stat)
        }
        if let stat = Self.stat(label: "FEELS LIKE LOW", current: Self.feelsLike(temp: forecast?.low, wind: forecast?.windSpeedAvg), normal: today.feelsLikeLow, unit: "°", step: tempStep, decimals: decimals) {
            stats.append(stat)
        }
        if let stat = windStat(decimals: decimals) {
            stats.append(stat)
        }
        if let stat = Self.stat(label: "GUSTS", current: forecast?.hourlyMaximumGustMax, normal: today.windGust, unit: " m/s", step: unitStep, decimals: decimals) {
            stats.append(stat)
        }
        if let stat = Self.stat(label: "HUMIDITY", current: currentHumidity, normal: today.humidityNowNormal ?? today.humidityAvg, unit: "%", step: unitStep, decimals: decimals) {
            stats.append(stat)
        }
        let precip = normals.precipitation
        if let observed = precip?.todayObservedMm,
           var stat = Self.stat(label: "PRECIP TODAY", current: observed, normal: precip?.todayNormalMm ?? today.precipMm, unit: " mm", step: 0.1, decimals: 1)
        {
            if let forecastMm = forecast?.precipitationMm {
                stat.detail += " · forecast \(Self.format(forecastMm, decimals: 1)) mm"
            }
            stats.append(stat)
        } else if var stat = Self.stat(label: "PRECIP", current: forecast?.precipitationMm, normal: today.precipMm, unit: " mm", step: 0.1, decimals: 1) {
            if let wetDays = today.precipDaysPct {
                stat.detail += " · rain on \(Self.format(wetDays, decimals: 0))% of days"
            }
            stats.append(stat)
        }
        if let precip, var stat = Self.stat(label: "MONTH TO DATE", current: precip.monthToDateObservedMm, normal: precip.monthToDateNormalMm, unit: " mm", step: 0.1, decimals: 1) {
            if let month = precip.monthNormalMm {
                stat.detail += " · full month \(Self.format(month, decimals: 0)) mm"
            }
            stats.append(stat)
        }
        if let snowNormal = today.snowCm, snowNormal >= 0.5 || (currentSnowDepth ?? 0) > 0,
           let stat = Self.stat(label: "SNOW", current: currentSnowDepth, normal: snowNormal, unit: " cm", step: unitStep, decimals: decimals)
        {
            stats.append(stat)
        }
        return stats
    }

    /// Compact card row: high, low, feels-like now, wind.
    func summaryStats(decimals: Int) -> [DailyNormalStat] {
        let today = normals.today
        return [
            highStat(decimals: decimals),
            lowStat(decimals: decimals),
            Self.stat(label: "FEELS LIKE", current: currentFeelsLike, normal: today.feelsLikeNowNormal ?? today.feelsLikeAvg, unit: "°", step: decimals > 0 ? 0.1 : 0.5, decimals: decimals),
            windStat(decimals: decimals),
        ].compactMap { $0 }
    }

    private func highStat(decimals: Int) -> DailyNormalStat? {
        Self.stat(label: "HIGH", current: todayWeatherHigh, normal: normals.today.tempHigh, unit: "°", step: decimals > 0 ? 0.1 : 0.5, decimals: decimals)
    }

    private func lowStat(decimals: Int) -> DailyNormalStat? {
        Self.stat(label: "LOW", current: todayWeatherLow, normal: normals.today.tempLow, unit: "°", step: decimals > 0 ? 0.1 : 0.5, decimals: decimals)
    }

    private func windStat(decimals: Int) -> DailyNormalStat? {
        Self.stat(label: "WIND", current: currentWind, normal: normals.today.windNowNormal ?? normals.today.windAvg, unit: " m/s", step: Self.unitStep(decimals), decimals: decimals)
    }

    private static func unitStep(_ decimals: Int) -> Double {
        decimals > 0 ? 0.1 : 1
    }

    private static func stat(label: String, current: Double?, normal: Double?, unit: String, step: Double, decimals: Int) -> DailyNormalStat? {
        guard let normal else { return nil }
        guard let current else {
            return DailyNormalStat(label: label, value: format(normal, decimals: decimals) + unit, detail: "normal", delta: nil, unit: unit, step: step)
        }
        return DailyNormalStat(
            label: label,
            value: format(current, decimals: decimals) + unit,
            detail: "normal \(format(normal, decimals: decimals))\(unit)",
            delta: current - normal,
            unit: unit,
            step: step
        )
    }

    static func format(_ value: Double, decimals: Int) -> String {
        String(format: "%.\(decimals)f", value)
    }

    // MARK: - Chart Data

    private var localCalendar: Calendar {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = timeZone
        return calendar
    }

    func dailyChartData(withRange: Bool, withForecastMarkers: Bool) -> NormalsTemperatureChart.Data {
        let calendar = localCalendar
        let now = Date()
        let month = calendar.component(.month, from: now)
        let todayDay = calendar.component(.day, from: now)

        let days = normals.daily
            .filter { $0.month == month }
            .sorted { $0.day < $1.day }
        let avgs = days.map { $0.tempAvg ?? 0 }
        let highs = withRange ? days.map { $0.tempHigh ?? $0.tempAvg ?? 0 } : []
        let lows = withRange ? days.map { $0.tempLow ?? $0.tempAvg ?? 0 } : []
        let todayIndex = min(max(todayDay - 1, 0), max(days.count - 1, 0))

        return NormalsTemperatureChart.Data(
            highs: highs,
            lows: lows,
            avgs: avgs,
            xLabels: NormalsTemperatureChart.Data.dayLabels(count: days.count),
            todayIndex: todayIndex,
            currentTemp: currentTemp,
            todayWeatherHigh: withForecastMarkers ? todayWeatherHigh : nil,
            todayWeatherLow: withForecastMarkers ? todayWeatherLow : nil
        )
    }

    /// The hourly mean with its 10th–90th percentile band, rotated from UTC to
    /// local hours.
    var hourlyChartData: NormalsTemperatureChart.Data {
        let now = Date()
        let offsetHours = Int((Double(timeZone.secondsFromGMT(for: now)) / 3600).rounded())
        let localized = { (utc: [Double]?) -> [Double] in
            guard let utc, utc.count == 24 else { return [] }
            return (0..<24).map { hour in utc[((hour - offsetHours) % 24 + 24) % 24] }
        }
        let currentHour = localCalendar.component(.hour, from: now)

        return NormalsTemperatureChart.Data(
            highs: localized(normals.today.tempHourlyP90),
            lows: localized(normals.today.tempHourlyP10),
            avgs: localized(normals.today.tempHourly),
            xLabels: NormalsTemperatureChart.Data.hourLabels(),
            todayIndex: currentHour,
            currentTemp: currentTemp,
            todayWeatherHigh: nil,
            todayWeatherLow: nil
        )
    }

    // MARK: - Formatting

    static func rounded(_ value: Double, step: Double) -> Double {
        (value / step).rounded() * step
    }

    static func deltaText(_ diff: Double, unit: String) -> String {
        if diff > 0 {
            return "+\(formatNumber(diff))\(unit)"
        } else if diff < 0 {
            return "−\(formatNumber(-diff))\(unit)"
        }
        return "±0\(unit)"
    }

    static func deltaColor(_ diff: Double) -> Color {
        if diff > 0 { return .red }
        if diff < 0 { return .blue }
        return .secondary
    }

    static func formatNumber(_ value: Double) -> String {
        if value == value.rounded() {
            return "\(Int(value))"
        }
        return String(format: "%.1f", value)
    }
}

// MARK: - Preview Data

extension DailyNormalsComparison {
    static var preview: DailyNormalsComparison {
        let daily: [DailyNormal] = (1...12).flatMap { month in
            (1...31).map { day in
                let doy = Double((month - 1) * 30 + day)
                let avg = 5 - 12 * cos(2 * .pi * (doy - 15) / 365)
                return DailyNormal(
                    month: month, day: day, tempAvg: avg, tempHigh: avg + 3.5, tempLow: avg - 3.2,
                    feelsLikeAvg: avg - 1, feelsLikeHigh: avg + 2.5, feelsLikeLow: avg - 4.5,
                    windAvg: 4.1, windGust: 9.8, humidityAvg: 78, precipMm: 2, precipDaysPct: 48, snowCm: month <= 3 ? 8 : 0
                )
            }
        }
        let hourly = (0..<24).map { hour in 14.3 + 2.4 * sin(2 * .pi * Double(hour - 9) / 24) }
        return DailyNormalsComparison(
            normals: DailyClimateNormalsResponse(
                station: StationInfo(name: "Helsinki Kaisaniemi", distanceKm: 0.6),
                period: "1991-2020",
                today: DailyNormalToday(
                    month: 9, day: 3, tempAvg: 14.3, tempHigh: 17.7, tempLow: 11.3,
                    feelsLikeAvg: 13.6, feelsLikeHigh: 17.2, feelsLikeLow: 10.4,
                    windAvg: 4.1, windGust: 9.8, humidityAvg: 78,
                    precipMm: 2.3, precipDaysPct: 48, snowCm: 0,
                    tempHourly: hourly, tempHourlyP10: hourly.map { $0 - 2.8 }, tempHourlyP90: hourly.map { $0 + 3.1 },
                    feelsLikeHourly: hourly.map { $0 - 0.7 },
                    windHourly: Array(repeating: 4.1, count: 24), humidityHourly: Array(repeating: 78, count: 24),
                    tempNowNormal: 13.9, tempDiff: 0.1,
                    feelsLikeNowNormal: 13.2, windNowNormal: 4.1, humidityNowNormal: 78
                ),
                precipitation: DailyNormalsPrecipitation(
                    station: StationInfo(name: "Espoo Tapiola", distanceKm: 2.1),
                    todayObservedMm: 0.2, todayNormalMm: 2.3, monthToDateObservedMm: 3.1,
                    monthToDateNormalMm: 9.2, monthNormalMm: 66, observedThrough: Date()
                ),
                daily: daily
            ),
            currentTemp: 14,
            currentFeelsLike: 11.5,
            currentWind: 6,
            currentHumidity: 71,
            currentSnowDepth: nil,
            todayForecast: DailyForecast(
                date: "2026-09-03", high: 18.7, low: 13.9, temperatureAvg: 15.8, symbol: nil,
                windSpeedAvg: 5.2, humidityAvg: 74, precipitationMm: 0.4, hourlyMaximumGustMax: 11.5
            ),
            timeZone: TimeZone(identifier: "Europe/Helsinki") ?? .current
        )
    }
}
