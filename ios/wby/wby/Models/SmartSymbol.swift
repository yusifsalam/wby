import Foundation
import UIKit

enum SmartSymbol {
    static func systemImageName(for rawCode: String?) -> String {
        guard let parsed = parsedCode(from: rawCode) else { return "cloud.fill" }
        let isNight = parsed >= 100
        let code = normalizeNightOffset(parsed)

        switch code {
        case 1:
            return firstAvailable(isNight ? ["moon.stars.fill", "moon.fill", "sun.max.fill"] : ["sun.max.fill"])
        case 2, 4, 6:
            return firstAvailable(isNight ? ["cloud.moon.fill", "cloud.fill"] : ["cloud.sun.fill", "cloud.fill"])
        case 7:
            return "cloud.fill"
        case 9:
            return firstAvailable(["cloud.fog.fill", "cloud.fill"])

        case 71, 74, 77:
            return firstAvailable(isNight ? ["cloud.moon.rain.fill", "cloud.bolt.rain.fill"] : ["cloud.bolt.rain.fill"])

        case 21, 24, 27:
            return firstAvailable(isNight ? ["cloud.moon.rain.fill", "cloud.rain.fill"] : ["cloud.rain.fill"])
        case 11:
            return firstAvailable(isNight ? ["cloud.moon.rain.fill", "cloud.drizzle.fill"] : ["cloud.drizzle.fill"])
        case 14, 17:
            return "cloud.sleet.fill"

        case 31, 32, 33, 34, 35, 36, 37, 38, 39:
            return firstAvailable(isNight ? ["cloud.moon.rain.fill", "cloud.rain.fill"] : ["cloud.rain.fill"])

        case 41, 42, 43, 44, 45, 46, 47, 48, 49:
            return "cloud.sleet.fill"

        case 51, 52, 53, 54, 55, 56, 57, 58, 59:
            return "cloud.snow.fill"

        case 61, 64, 67:
            return firstAvailable(["cloud.hail.fill", "cloud.sleet.fill", "cloud.rain.fill"])

        default:
            return "cloud.fill"
        }
    }

    static func normalizedCode(from rawCode: String?) -> Int? {
        guard let parsed = parsedCode(from: rawCode) else { return nil }
        return normalizeNightOffset(parsed)
    }

    private static func normalizeNightOffset(_ code: Int) -> Int {
        // Official app source: symbol assets include separate night codes (101, 102...),
        // while translation keys normalize by subtracting 100.
        if code >= 100 {
            return code - 100
        }
        return code
    }

    private static func parsedCode(from rawCode: String?) -> Int? {
        guard let rawCode else { return nil }
        return Int(rawCode)
    }

    private static func firstAvailable(_ candidates: [String]) -> String {
        for name in candidates where UIImage(systemName: name) != nil {
            return name
        }
        return "cloud.fill"
    }
}
