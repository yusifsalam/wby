import Foundation
import SwiftUI

enum SmartSymbol {
    static func systemImageName(for rawCode: String?) -> String {
        guard let parsed = parsedCode(from: rawCode) else { return "cloud.fill" }
        let isNight = parsed >= 100
        let code = normalizeNightOffset(parsed)

        switch code {
        case 1:
            return isNight ? "moon.stars.fill" : "sun.max.fill"
        case 2, 4, 6:
            return isNight ? "cloud.moon.fill" : "cloud.sun.fill"
        case 7:
            return "cloud.fill"
        case 9:
            return "cloud.fog.fill"

        case 11:
            return isNight ? "cloud.moon.rain.fill" : "cloud.drizzle.fill"
        case 14, 17:
            return "cloud.sleet.fill"

        case 21, 24, 27:
            return isNight ? "cloud.moon.rain.fill" : "cloud.rain.fill"

        case 31...39:
            return isNight ? "cloud.moon.rain.fill" : "cloud.rain.fill"

        case 41...49:
            return "cloud.sleet.fill"

        case 51...59:
            return "cloud.snow.fill"

        case 61, 64, 67:
            return "cloud.hail.fill"

        case 71, 74, 77:
            return isNight ? "cloud.moon.bolt.fill" : "cloud.bolt.rain.fill"

        default:
            return "cloud.fill"
        }
    }

    static func normalizedCode(from rawCode: String?) -> Int? {
        guard let parsed = parsedCode(from: rawCode) else { return nil }
        return normalizeNightOffset(parsed)
    }

    private static func normalizeNightOffset(_ code: Int) -> Int {
        // FMI symbol assets use separate night codes (101, 102...),
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
}

#if DEBUG
private struct SmartSymbolGallery: View {
    static let dayCodes: [Int] = [
        1, 2, 4, 6, 7, 9,
        11, 14, 17,
        21, 24, 27,
        31, 32, 33, 34, 35, 36, 37, 38, 39,
        41, 42, 43, 44, 45, 46, 47, 48, 49,
        51, 52, 53, 54, 55, 56, 57, 58, 59,
        61, 64, 67,
        71, 74, 77
    ]

    let columns = [GridItem(.adaptive(minimum: 120), spacing: 12)]

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                section(title: "Day", codes: Self.dayCodes, isNight: false)
                section(title: "Night", codes: Self.dayCodes.map { $0 + 100 }, isNight: true)
            }
            .padding()
        }
    }

    private func section(title: String, codes: [Int], isNight: Bool) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(title).font(.title2.bold())
            LazyVGrid(columns: columns, spacing: 12) {
                ForEach(codes, id: \.self) { code in
                    cell(code: code, isNight: isNight)
                }
            }
        }
    }

    private func cell(code: Int, isNight: Bool) -> some View {
        let raw = String(code)
        let symbol = SmartSymbol.systemImageName(for: raw)
        let label = WeatherSymbols.conditionDescription(from: raw) ?? "—"
        return VStack(spacing: 6) {
            Image(systemName: symbol)
                .symbolRenderingMode(.multicolor)
                .font(.system(size: 36))
                .frame(height: 44)
            Text("\(code)").font(.caption.monospacedDigit().bold())
            Text(label)
                .font(.caption2)
                .multilineTextAlignment(.center)
                .lineLimit(3)
            Text(symbol)
                .font(.system(size: 9, design: .monospaced))
                .foregroundStyle(.white.opacity(0.75))
                .lineLimit(1)
                .truncationMode(.middle)
        }
        .frame(maxWidth: .infinity)
        .padding(10)
        .background(
            RoundedRectangle(cornerRadius: 12, style: .continuous)
                .fill(isNight
                      ? Color(red: 0.05, green: 0.08, blue: 0.18)
                      : Color(red: 0.45, green: 0.68, blue: 0.92))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 12, style: .continuous)
                .stroke(Color.primary.opacity(0.08))
        )
        .foregroundStyle(.white)
    }
}

#Preview("Smart Symbols Gallery") {
    SmartSymbolGallery()
}
#endif
