import CoreLocation
import SwiftUI

struct SunriseCard: View {
    let coordinate: CLLocationCoordinate2D
    let referenceDate: Date
    let elevationMeters: Double?

    private var sunTimes: (sunrise: Date?, sunset: Date?) {
        Self.calculateSunTimes(
            date: referenceDate,
            latitude: coordinate.latitude,
            longitude: coordinate.longitude,
            timeZone: .current,
            elevationMeters: elevationMeters ?? 0
        )
    }

    private var progress: CGFloat {
        guard let sunrise = sunTimes.sunrise, let sunset = sunTimes.sunset else { return 0.5 }
        let total = sunset.timeIntervalSince(sunrise)
        guard total > 0 else { return 0.5 }
        let value = referenceDate.timeIntervalSince(sunrise) / total
        return CGFloat(min(max(value, 0), 1))
    }

    var body: some View {
        HalfCard(
            title: "SUNRISE",
            icon: "sunrise",
            keyValue: timeText(sunTimes.sunrise),
            description: "Sunset: \(timeText(sunTimes.sunset))"
        ) {
            sunCurve
        }
    }

    private var sunCurve: some View {
        GeometryReader { geo in
            let width = geo.size.width
            let height = geo.size.height
            let midY = height * 0.58
            let amp = height * 0.33
            let dayStartX = width * 0.25
            let dayEndX = width * 0.75
            let markerX = dayStartX + (dayEndX - dayStartX) * progress
            let markerY = curveY(x: markerX, width: width, midY: midY, amplitude: amp)

            ZStack {
                Path { p in
                    p.move(to: CGPoint(x: 0, y: midY))
                    p.addLine(to: CGPoint(x: width, y: midY))
                }
                .stroke(Color.primary.opacity(0.35), lineWidth: 2)

                Path { p in
                    p.move(to: CGPoint(x: 0, y: curveY(x: 0, width: width, midY: midY, amplitude: amp)))
                    let step = max(width / 80, 1)
                    var x: CGFloat = step
                    while x <= width {
                        p.addLine(to: CGPoint(x: x, y: curveY(x: x, width: width, midY: midY, amplitude: amp)))
                        x += step
                    }
                }
                .stroke(Color.primary.opacity(0.14), style: StrokeStyle(lineWidth: 4, lineCap: .round))

                Path { p in
                    p.move(to: CGPoint(x: dayStartX, y: curveY(x: dayStartX, width: width, midY: midY, amplitude: amp)))
                    let step = max((dayEndX - dayStartX) / 40, 1)
                    var x = dayStartX + step
                    while x <= dayEndX {
                        p.addLine(to: CGPoint(x: x, y: curveY(x: x, width: width, midY: midY, amplitude: amp)))
                        x += step
                    }
                }
                .stroke(Color.primary.opacity(0.24), style: StrokeStyle(lineWidth: 4, lineCap: .round))

                Circle()
                    .fill(Color(red: 0.22, green: 0.24, blue: 0.32))
                    .frame(width: 12, height: 12)
                    .overlay(Circle().stroke(Color.primary.opacity(0.75), lineWidth: 1.2))
                    .position(x: markerX, y: markerY)
            }
        }
        .frame(height: 40)
    }

    private func curveY(x: CGFloat, width: CGFloat, midY: CGFloat, amplitude: CGFloat) -> CGFloat {
        guard width > 0 else { return midY }
        let angle = (x / width) * .pi * 2
        return midY + amplitude * cos(angle)
    }

    private func timeText(_ date: Date?) -> String {
        guard let date else { return "--.--" }
        let formatter = DateFormatter()
        formatter.dateFormat = "H.mm"
        formatter.timeZone = .current
        return formatter.string(from: date)
    }

    static func isNight(coordinate: CLLocationCoordinate2D, date: Date, elevationMeters: Double) -> Bool {
        let times = calculateSunTimes(
            date: date,
            latitude: coordinate.latitude,
            longitude: coordinate.longitude,
            timeZone: .current,
            elevationMeters: elevationMeters
        )
        guard let sunrise = times.sunrise, let sunset = times.sunset else { return false }
        return date < sunrise || date > sunset
    }

    private static func calculateSunTimes(
        date: Date,
        latitude: Double,
        longitude: Double,
        timeZone: TimeZone,
        elevationMeters: Double
    ) -> (sunrise: Date?, sunset: Date?) {
        var localCalendar = Calendar(identifier: .gregorian)
        localCalendar.timeZone = timeZone
        let localDayStart = localCalendar.startOfDay(for: date)
        guard let localNoon = localCalendar.date(byAdding: .hour, value: 12, to: localDayStart),
              let (sunriseTS, sunsetTS) = calculateNOAASunTimes(
                  referenceTimestamp: localNoon.timeIntervalSince1970,
                  latitude: latitude,
                  longitude: longitude,
                  elevationMeters: elevationMeters
              ) else {
            return (nil, nil)
        }
        return (
            roundedToNearestMinute(Date(timeIntervalSince1970: sunriseTS)),
            roundedToNearestMinute(Date(timeIntervalSince1970: sunsetTS))
        )
    }

    private static func calculateNOAASunTimes(
        referenceTimestamp: Double,
        latitude: Double,
        longitude: Double,
        elevationMeters: Double
    ) -> (sunriseTS: Double, sunsetTS: Double)? {
        let jDate = timestampToJulianDay(referenceTimestamp)

        // East-longitude positive, matching CLLocationCoordinate2D.
        let n = ceil(jDate - (j2000JulianDay + 0.0009) + leapSecondCorrectionDays)
        let jStar = n + 0.0009 - longitude / 360.0

        let meanAnomaly = deg2rad(normalizeDegrees(357.5291 + 0.98560028 * jStar))
        let center = 1.9148 * sin(meanAnomaly)
            + 0.02 * sin(2 * meanAnomaly)
            + 0.0003 * sin(3 * meanAnomaly)
        let eclipticLongitude = deg2rad(normalizeDegrees(rad2deg(meanAnomaly) + center + 180.0 + 102.9372))

        let jTransit = j2000JulianDay
            + jStar
            + 0.0053 * sin(meanAnomaly)
            - 0.0069 * sin(2 * eclipticLongitude)

        let latitudeRad = deg2rad(latitude)
        let sinDeclination = sin(eclipticLongitude) * sin(deg2rad(23.4397))
        let cosDeclination = cos(asin(sinDeclination))

        let elevationDip = 2.076 * sqrt(max(elevationMeters, 0)) / 60.0
        let solarElevationAtEvent = -(sunriseRefractionDegrees + elevationDip)
        let cosHourAngle = (
            sin(deg2rad(solarElevationAtEvent)) - sin(latitudeRad) * sinDeclination
        ) / (cos(latitudeRad) * cosDeclination)

        guard cosHourAngle >= -1.0, cosHourAngle <= 1.0 else {
            return nil
        }
        let hourAngle = acos(cosHourAngle)

        let sunriseJD = jTransit - hourAngle / (2.0 * .pi)
        let sunsetJD = jTransit + hourAngle / (2.0 * .pi)
        return (julianDayToTimestamp(sunriseJD), julianDayToTimestamp(sunsetJD))
    }

    private static func roundedToNearestMinute(_ date: Date) -> Date {
        Date(timeIntervalSince1970: (date.timeIntervalSince1970 / 60.0).rounded() * 60.0)
    }

    private static func timestampToJulianDay(_ timestamp: Double) -> Double {
        timestamp / 86400.0 + unixEpochJulianDay
    }

    private static func julianDayToTimestamp(_ julianDay: Double) -> Double {
        (julianDay - unixEpochJulianDay) * 86400.0
    }

    private static let unixEpochJulianDay = 2440587.5
    private static let j2000JulianDay = 2451545.0
    private static let leapSecondCorrectionDays = 69.184 / 86400.0
    private static let sunriseRefractionDegrees = 0.833
    private static func deg2rad(_ value: Double) -> Double { value * .pi / 180.0 }
    private static func rad2deg(_ value: Double) -> Double { value * 180.0 / .pi }

    private static func normalizeDegrees(_ value: Double) -> Double {
        let normalized = value.truncatingRemainder(dividingBy: 360.0)
        return normalized < 0 ? normalized + 360.0 : normalized
    }
}

#Preview {
    ZStack {
        Color.blue.opacity(0.4).ignoresSafeArea()
        SunriseCard(
            coordinate: CLLocationCoordinate2D(latitude: 60.1699, longitude: 24.9384),
            referenceDate: .now,
            elevationMeters: 32
        )
        .padding()
    }
}
