import SwiftUI

struct WindCard: View {
    let current: CurrentConditions

    var body: some View {
        FullCard(
            title: "WIND",
            icon: "wind",
            rows: [
                ("Wind", speedText(current.resolvedWindSpeed)),
                ("Gusts", speedText(current.resolvedWindGust)),
                ("Direction", directionText(current.resolvedWindDirection)),
            ]
        ) {
            dial
        }
    }

    private var dial: some View {
        ZStack {
            tickRing
            directionVector
                .zIndex(0)
            centerPlate
                .zIndex(1)

            VStack(spacing: -2) {
                Text(speedNumber(current.resolvedWindSpeed))
                    .font(.system(size: 22, weight: .light))
                    .foregroundStyle(.primary)
                Text("m/s")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(.primary)
            }
            .zIndex(2)

            cardinalMark("N", x: 0, y: -58)
            cardinalMark("E", x: 58, y: 0)
            cardinalMark("S", x: 0, y: 58)
            cardinalMark("W", x: -58, y: 0)
        }
        .frame(width: 136, height: 136)
    }

    private var centerPlate: some View {
        Circle()
            .fill(.clear)
            .frame(width: 68, height: 68)
            .background(.ultraThinMaterial.opacity(0.38), in: Circle())
            .overlay(
                Circle()
                    .stroke(Color.primary.opacity(0.12), lineWidth: 1)
            )
    }

    private var tickRing: some View {
        ZStack {
            ForEach(0..<72, id: \.self) { index in
                let major = index % 6 == 0
                RoundedRectangle(cornerRadius: 1)
                    .fill(Color.primary.opacity(major ? 0.24 : 0.14))
                    .frame(width: major ? 1.5 : 1, height: major ? 10 : 6)
                    .offset(y: major ? -62 : -60)
                    .rotationEffect(.degrees(Double(index) * 5))
            }
        }
    }

    private var directionVector: some View {
        // FMI direction is where wind comes FROM (0=N, 90=E).
        // Arrow default orientation is ← (West = 270°). To point where wind goes TO
        // (FROM - 180°), rotation = (windDirection - 180°) - 270° = windDirection - 90°.
        let angle = (current.resolvedWindDirection ?? 0) - 90
        return ZStack {
            Capsule()
                .fill(Color.primary)
                .frame(width: 28, height: 3)
                .offset(x: -46)
            Capsule()
                .fill(Color.primary)
                .frame(width: 34, height: 3)
                .offset(x: 32)
            Image(systemName: "arrowtriangle.left.fill")
                .font(.system(size: 14))
                .foregroundStyle(.primary)
                .offset(x: -50)
            Circle()
                .fill(Color.primary)
                .frame(width: 14, height: 14)
                .offset(x: 52)
        }
        .rotationEffect(.degrees(angle))
        .frame(width: 136, height: 136)
        .mask {
            Circle()
                .stroke(lineWidth: 68)
                .frame(width: 136, height: 136)
        }
    }

    private func cardinalMark(_ letter: String, x: CGFloat, y: CGFloat) -> some View {
        Text(letter)
            .font(.system(size: 14, weight: .bold))
            .foregroundStyle(.secondary)
            .offset(x: x, y: y)
    }

    private func speedText(_ speed: Double?) -> String {
        guard let speed else { return "-- m/s" }
        return "\(Int(speed.rounded())) m/s"
    }

    private func speedNumber(_ speed: Double?) -> String {
        guard let speed else { return "--" }
        return "\(Int(speed.rounded()))"
    }

    private func directionText(_ degrees: Double?) -> String {
        guard let degrees else { return "--" }
        return "\(Int(degrees.rounded()))° \(cardinalDirection(for: degrees))"
    }

    private func cardinalDirection(for degrees: Double) -> String {
        let sectors = ["N", "NE", "E", "SE", "S", "SW", "W", "NW"]
        let index = Int((degrees + 22.5) / 45.0) % sectors.count
        return sectors[index]
    }
}

#Preview {
    ZStack {
        Color.blue.opacity(0.45).ignoresSafeArea()
        WindCard(
            current: CurrentConditions(
                temperature: -6.0,
                feelsLike: -11.0,
                windSpeed: 1.0,
                windGust: 2.0,
                windDirection: 353.0,
                humidity: 84.0,
                pressure: 1012.0,
                observedAt: .now
            )
        )
        .padding()
    }
}
