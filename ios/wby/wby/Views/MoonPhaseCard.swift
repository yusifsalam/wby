import CoreLocation
import SwiftUI

struct MoonPhaseCard: View {
    let coordinate: CLLocationCoordinate2D
    let referenceDate: Date

    private var data: MoonData {
        Self.calculateMoonData(
            date: referenceDate,
            latitude: coordinate.latitude,
            longitude: coordinate.longitude
        )
    }

    var body: some View {
        FullCard(
            title: data.phaseName,
            icon: data.sfSymbolName,
            rows: [
                ("Illumination", "\(Int((data.illumination * 100).rounded())) %"),
                ("Next Moonset", data.moonsetText),
                ("Next Full Moon", data.daysToFullMoonText),
            ]
        ) {
            Image(systemName: data.sfSymbolName)
                .symbolRenderingMode(.palette)
                .foregroundStyle(.white, Color(white: 0.35))
                .font(.system(size: 110))
                .frame(width: 120, height: 120)
        }
    }

    // MARK: - Data model

    private struct MoonData {
        let phaseName: String
        let sfSymbolName: String
        let illumination: Double
        let moonset: Date?
        let daysToFullMoon: Double

        var moonsetText: String {
            guard let moonset else { return "--:--" }
            let formatter = DateFormatter()
            formatter.dateFormat = "H.mm"
            formatter.timeZone = .current
            return formatter.string(from: moonset)
        }

        var daysToFullMoonText: String {
            let days = Int(daysToFullMoon.rounded())
            if days == 0 { return "TODAY" }
            return "\(days) \(days == 1 ? "DAY" : "DAYS")"
        }
    }

    // MARK: - Moon calculations

    private static func calculateMoonData(date: Date, latitude: Double, longitude: Double) -> MoonData {
        let jd = date.timeIntervalSince1970 / 86400.0 + unixEpochJD

        let phaseFraction = moonPhaseFraction(jd: jd)
        let illumination = (1.0 - cos(phaseFraction * 2.0 * .pi)) / 2.0
        let synodicPeriod = 29.530588853
        let daysToFull: Double = phaseFraction < 0.5
            ? (0.5 - phaseFraction) * synodicPeriod
            : (1.5 - phaseFraction) * synodicPeriod

        let (phaseName, sfSymbol) = phaseInfo(for: phaseFraction)

        let moonsetJD = findNextMoonset(fromJD: jd, latitude: latitude, longitude: longitude)
        let moonset = moonsetJD.map { Date(timeIntervalSince1970: ($0 - unixEpochJD) * 86400.0) }

        return MoonData(
            phaseName: phaseName,
            sfSymbolName: sfSymbol,
            illumination: illumination,
            moonset: moonset,
            daysToFullMoon: daysToFull
        )
    }

    /// Phase fraction [0,1) derived from the actual Sun–Moon elongation angle.
    /// 0 = new moon, 0.5 = full moon. More accurate than dividing elapsed time
    /// by the mean synodic period, which ignores orbital eccentricity (±6 h variation).
    private static func moonPhaseFraction(jd: Double) -> Double {
        let moonLon = moonEclipticLongitudeDeg(jd: jd)
        let sunLon  = sunEclipticLongitudeDeg(jd: jd)
        return normDeg(moonLon - sunLon) / 360.0
    }

    private static func moonEclipticLongitudeDeg(jd: Double) -> Double {
        moonEclipticFull(jd: jd).lambda
    }

    private static func sunEclipticLongitudeDeg(jd: Double) -> Double {
        let T  = (jd - 2451545.0) / 36525.0
        let Ls = normDeg(280.46646 + 36000.76983 * T)
        let Ms = normDeg(357.52911 + 35999.05029 * T - 0.0001537 * T * T)
        let C  = (1.914602 - 0.004817 * T - 0.000014 * T * T) * sin(deg2rad(Ms))
               + (0.019993 - 0.000101 * T) * sin(deg2rad(2 * Ms))
               + 0.000289 * sin(deg2rad(3 * Ms))
        return normDeg(Ls + C)
    }

    private static func phaseInfo(for fraction: Double) -> (String, String) {
        switch fraction {
        case 0.0..<0.025, 0.975...: return ("NEW MOON",        "moonphase.new.moon")
        case 0.025..<0.225:         return ("WAXING CRESCENT", "moonphase.waxing.crescent")
        case 0.225..<0.275:         return ("FIRST QUARTER",   "moonphase.first.quarter")
        case 0.275..<0.475:         return ("WAXING GIBBOUS",  "moonphase.waxing.gibbous")
        case 0.475..<0.525:         return ("FULL MOON",       "moonphase.full.moon")
        case 0.525..<0.725:         return ("WANING GIBBOUS",  "moonphase.waning.gibbous")
        case 0.725..<0.775:         return ("LAST QUARTER",    "moonphase.last.quarter")
        default:                    return ("WANING CRESCENT", "moonphase.waning.crescent")
        }
    }

    // MARK: - Moon position

    /// Moon's geocentric altitude above the horizon in degrees at the given JD, lat, lon.
    private static func moonAltitudeDeg(jd: Double, latitudeDeg: Double, longitudeDeg: Double) -> Double {
        let (ra, dec, _) = moonEquatorialCoords(jd: jd)
        let T    = (jd - 2451545.0) / 36525.0
        let gmst = deg2rad(normDeg(280.46061837 + 360.98564736629 * (jd - 2451545.0)
                                 + 0.000387933 * T * T))
        let lha  = gmst + deg2rad(longitudeDeg) - ra
        let latRad = deg2rad(latitudeDeg)
        let sinAlt = sin(dec) * sin(latRad) + cos(dec) * cos(latRad) * cos(lha)
        return rad2deg(asin(max(-1.0, min(1.0, sinAlt))))
    }

    /// Geocentric equatorial coordinates (RA in radians, Dec in radians, distance in km).
    /// Uses the full Meeus Chapter 47 series.
    private static func moonEquatorialCoords(jd: Double) -> (ra: Double, dec: Double, delta: Double) {
        let (lambdaDeg, betaDeg, delta) = moonEclipticFull(jd: jd)
        let lambda = deg2rad(lambdaDeg)
        let beta   = deg2rad(betaDeg)
        let T      = (jd - 2451545.0) / 36525.0
        // Mean obliquity of the ecliptic (Meeus, accurate to 0.001")
        let eps = deg2rad(23.4392911 - 0.013004167 * T - 0.000000164 * T * T + 0.000000504 * T * T * T)
        let ra  = atan2(sin(lambda) * cos(eps) - tan(beta) * sin(eps), cos(lambda))
        let dec = asin(max(-1.0, min(1.0, sin(beta) * cos(eps) + cos(beta) * sin(eps) * sin(lambda))))
        return (ra, dec, delta)
    }

    /// Full Meeus Chapter 47 Moon position.
    /// Returns ecliptic longitude λ (degrees), latitude β (degrees), and distance Δ (km).
    private static func moonEclipticFull(jd: Double) -> (lambda: Double, beta: Double, delta: Double) {
        let T = (jd - 2451545.0) / 36525.0

        // Fundamental arguments (Meeus p. 338)
        let Lp = normDeg(218.3164477  + 481267.88123421 * T - 0.0015786 * T*T + T*T*T / 538841.0  - T*T*T*T / 65194000.0)
        let D  = normDeg(297.8501921  + 445267.1114034  * T - 0.0018819 * T*T + T*T*T / 545868.0  - T*T*T*T / 113065000.0)
        let M  = normDeg(357.5291092  +  35999.0502909  * T - 0.0001536 * T*T + T*T*T / 24490000.0)
        let Mp = normDeg(134.9633964  + 477198.8675055  * T + 0.0087414 * T*T + T*T*T / 69699.0   - T*T*T*T / 14712000.0)
        let F  = normDeg( 93.2720950  + 483202.0175233  * T - 0.0036539 * T*T - T*T*T / 3526000.0 + T*T*T*T / 863310000.0)
        let A1 = normDeg(119.75 + 131.849    * T)
        let A2 = normDeg( 53.09 + 479264.290 * T)
        let A3 = normDeg(313.45 + 481266.484 * T)
        let E  = 1.0 - 0.002516 * T - 0.0000074 * T * T

        // Table 47.A — longitude (Σl × 10⁻⁶ °) and distance (Σr × 10⁻³ km)
        // Columns: D  M  M′  F  Σl  Σr
        let lrTerms: [(Int8, Int8, Int8, Int8, Int32, Int32)] = [
            ( 0,  0,  1,  0,  6288774, -20905355),
            ( 2,  0, -1,  0,  1274027,  -3699111),
            ( 2,  0,  0,  0,   658314,  -2955968),
            ( 0,  0,  2,  0,   213618,   -569925),
            ( 0,  1,  0,  0,  -185116,     48888),
            ( 0,  0,  0,  2,  -114332,     -3149),
            ( 2,  0, -2,  0,    58793,    246158),
            ( 2, -1, -1,  0,    57066,   -152138),
            ( 2,  0,  1,  0,    53322,   -170733),
            ( 2, -1,  0,  0,    45758,   -204586),
            ( 0,  1, -1,  0,   -40923,   -129620),
            ( 1,  0,  0,  0,   -34720,    108743),
            ( 0,  1,  1,  0,   -30383,    104755),
            ( 2,  0,  0, -2,    15327,     10321),
            ( 0,  0,  1,  2,   -12528,         0),
            ( 0,  0,  1, -2,    10980,     79661),
            ( 4,  0, -1,  0,    10675,    -34782),
            ( 0,  0,  3,  0,    10034,    -23210),
            ( 4,  0, -2,  0,     8548,    -21636),
            ( 2,  1, -1,  0,    -7888,     24208),
            ( 2,  1,  0,  0,    -6766,     30824),
            ( 1,  0, -1,  0,    -5163,     -8379),
            ( 1,  1,  0,  0,     4987,    -16675),
            ( 2, -1,  1,  0,     4036,    -12831),
            ( 2,  0,  2,  0,     3994,    -10445),
            ( 4,  0,  0,  0,     3861,    -11650),
            ( 2,  0, -3,  0,     3665,     14403),
            ( 0,  1, -2,  0,    -2689,     -7003),
            ( 2,  0, -1,  2,    -2602,         0),
            ( 2, -1, -2,  0,     2390,     10056),
            ( 1,  0,  1,  0,    -2348,      6322),
            ( 2, -2,  0,  0,     2236,     -9884),
            ( 0,  1,  2,  0,    -2120,      5751),
            ( 0,  2,  0,  0,    -2069,         0),
            ( 2, -2, -1,  0,     2048,     -4950),
            ( 2,  0,  1, -2,    -1773,      4130),
            ( 2,  0,  0,  2,    -1595,         0),
            ( 4, -1, -1,  0,     1215,     -3958),
            ( 0,  0,  2,  2,    -1110,         0),
            ( 3,  0, -1,  0,     -892,      3258),
            ( 2,  1,  1,  0,     -810,      2616),
            ( 4, -1, -2,  0,      759,     -1897),
            ( 0,  2, -1,  0,     -713,     -2117),
            ( 2,  2, -1,  0,     -700,      2354),
            ( 2,  1, -2,  0,      691,         0),
            ( 2, -1,  0, -2,      596,         0),
            ( 4,  0,  1,  0,      549,     -1423),
            ( 0,  0,  4,  0,      537,     -1117),
            ( 4, -1,  0,  0,      520,     -1571),
            ( 1,  0, -2,  0,     -487,     -1739),
            ( 2,  1,  0, -2,     -399,         0),
            ( 0,  0,  2, -2,     -381,     -4421),
            ( 1,  1,  1,  0,      351,         0),
            ( 3,  0, -2,  0,     -340,         0),
            ( 4,  0, -3,  0,      330,         0),
            ( 2, -1,  2,  0,      327,         0),
            ( 0,  2,  1,  0,     -323,      1165),
            ( 1,  1, -1,  0,      299,         0),
            ( 2,  0,  3,  0,      294,         0),
            ( 2,  0, -1, -2,        0,      8752),
        ]

        var sigmaL: Double = 0
        var sigmaR: Double = 0
        for (dM, mM, mpM, fM, lC, rC) in lrTerms {
            let arg = Double(dM) * D + Double(mM) * M + Double(mpM) * Mp + Double(fM) * F
            let eF: Double = abs(mM) == 1 ? E : (abs(mM) == 2 ? E * E : 1.0)
            sigmaL += Double(lC) * eF * sin(deg2rad(arg))
            sigmaR += Double(rC) * eF * cos(deg2rad(arg))
        }

        // Table 47.B — latitude (Σb × 10⁻⁶ °)
        // Columns: D  M  M′  F  Σb
        let bTerms: [(Int8, Int8, Int8, Int8, Int32)] = [
            ( 0,  0,  0,  1,  5128122),
            ( 0,  0,  1,  1,   280602),
            ( 0,  0,  1, -1,   277693),
            ( 2,  0,  0, -1,   173237),
            ( 2,  0, -1,  1,    55413),
            ( 2,  0, -1, -1,    46271),
            ( 2,  0,  0,  1,    32573),
            ( 0,  0,  2,  1,    17198),
            ( 2,  0,  1, -1,     9266),
            ( 0,  0,  2, -1,     8822),
            ( 2, -1,  0, -1,     8216),
            ( 2,  0, -2, -1,     4324),
            ( 2,  0,  1,  1,     4200),
            ( 2,  1,  0, -1,    -3359),
            ( 2, -1, -1,  1,     2463),
            ( 2, -1,  0,  1,     2211),
            ( 2, -1, -1, -1,     2065),
            ( 0,  1, -1, -1,    -1870),
            ( 4,  0, -1, -1,     1828),
            ( 0,  1,  0,  1,    -1794),
            ( 0,  0,  0,  3,    -1749),
            ( 0,  1, -1,  1,    -1565),
            ( 1,  0,  0,  1,    -1491),
            ( 0,  1,  1,  1,    -1475),
            ( 0,  1,  1, -1,    -1410),
            ( 0,  1,  0, -1,    -1344),
            ( 1,  0,  0, -1,    -1335),
            ( 0,  0,  3,  1,     1107),
            ( 4,  0,  0, -1,     1021),
            ( 4,  0, -1,  1,      833),
            ( 0,  0,  1, -3,      777),
            ( 4,  0, -2,  1,      671),
            ( 2,  0,  0, -3,      607),
            ( 2,  0,  2, -1,      596),
            ( 2, -1,  1, -1,      491),
            ( 2,  0, -2,  1,     -451),
            ( 0,  0,  3, -1,      439),
            ( 2,  0,  2,  1,      422),
            ( 2,  0, -3, -1,      421),
            ( 2,  1, -1,  1,     -366),
            ( 2,  1,  0,  1,     -351),
            ( 4,  0,  0,  1,      331),
            ( 2, -1,  1,  1,      315),
            ( 2, -2,  0, -1,      302),
            ( 0,  0,  1,  3,     -283),
            ( 2,  1,  1, -1,     -229),
            ( 1,  1,  0, -1,      223),
            ( 1,  1,  0,  1,      223),
            ( 0,  1, -2, -1,     -220),
            ( 2,  1, -1, -1,     -220),
            ( 1,  0,  1,  1,     -185),
            ( 2, -1, -2, -1,      181),
            ( 0,  1,  2,  1,     -177),
            ( 4,  0, -2, -1,      176),
            ( 4, -1, -1, -1,      166),
            ( 1,  0,  1, -1,     -164),
            ( 4,  0,  1, -1,      132),
            ( 1,  0, -1, -1,     -119),
            ( 4, -1,  0, -1,      115),
            ( 2, -2,  0,  1,      107),
        ]

        var sigmaB: Double = 0
        for (dM, mM, mpM, fM, bC) in bTerms {
            let arg = Double(dM) * D + Double(mM) * M + Double(mpM) * Mp + Double(fM) * F
            let eF: Double = abs(mM) == 1 ? E : (abs(mM) == 2 ? E * E : 1.0)
            sigmaB += Double(bC) * eF * sin(deg2rad(arg))
        }

        // Additional corrections (Meeus p. 342)
        sigmaL +=  3958 * sin(deg2rad(A1))
                +  1962 * sin(deg2rad(Lp - F))
                +   318 * sin(deg2rad(A2))
        sigmaB += -2235 * sin(deg2rad(Lp))
                +   382 * sin(deg2rad(A3))
                +   175 * sin(deg2rad(A1 - F))
                +   175 * sin(deg2rad(A1 + F))
                +   127 * sin(deg2rad(Lp - Mp))
                -   115 * sin(deg2rad(Lp + Mp))

        let lambda = normDeg(Lp + sigmaL / 1_000_000.0)
        let beta   = sigmaB / 1_000_000.0
        let delta  = 385000.56 + sigmaR / 1000.0
        return (lambda, beta, delta)
    }

    /// Searches forward from startJD in 5-minute steps (up to 48 h) for the next
    /// downward horizon crossing of the moon. Returns nil if no set occurs (polar regions).
    private static func findNextMoonset(fromJD startJD: Double, latitude: Double, longitude: Double) -> Double? {
        // Horizon: parallax(Δ) − refraction(34') − semi-diameter(16'), using actual distance.
        // This varies with the Moon's distance (~54'–61'), unlike the fixed solar value of −0.833°.
        let delta   = moonEclipticFull(jd: startJD).delta
        let piMoon  = rad2deg(asin(6378.14 / delta))   // horizontal parallax
        let sdMoon  = rad2deg(asin(1737.4  / delta))   // semi-diameter
        let horizon = piMoon - 34.0 / 60.0 - sdMoon   // degrees

        let step = 5.0 / (24.0 * 60.0)  // 5-minute steps in JD

        var prevAlt = moonAltitudeDeg(jd: startJD, latitudeDeg: latitude, longitudeDeg: longitude)
        for i in 1...576 {  // 576 × 5 min = 48 h
            let jd  = startJD + Double(i) * step
            let alt = moonAltitudeDeg(jd: jd, latitudeDeg: latitude, longitudeDeg: longitude)
            if prevAlt > horizon && alt <= horizon {
                let frac = (prevAlt - horizon) / (prevAlt - alt)
                return startJD + (Double(i - 1) + frac) * step
            }
            prevAlt = alt
        }
        return nil
    }

    // MARK: - Constants & helpers

    private static let unixEpochJD  = 2440587.5

    private static func deg2rad(_ d: Double) -> Double { d * .pi / 180.0 }
    private static func rad2deg(_ r: Double) -> Double { r * 180.0 / .pi }
    private static func normDeg(_ d: Double) -> Double {
        let n = d.truncatingRemainder(dividingBy: 360.0)
        return n < 0 ? n + 360.0 : n
    }
}

#Preview {
    ZStack {
        Color(red: 0.11, green: 0.33, blue: 0.73).ignoresSafeArea()
        MoonPhaseCard(
            coordinate: CLLocationCoordinate2D(latitude: 60.1699, longitude: 24.9384),
            referenceDate: .now
        )
        .padding()
    }
}
