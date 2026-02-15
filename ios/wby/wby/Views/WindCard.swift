import SwiftUI

struct WindCard: View {
    let current: CurrentConditions
    let gustSpeed: Double?

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Label("WIND", systemImage: "wind")
                .font(.caption)
                .foregroundStyle(.white.opacity(0.8))

            HStack(alignment: .center, spacing: 22) {
                metricsColumn
                dial
            }
        }
        .padding()
        .background(cardBackground)
    }

    private var metricsColumn: some View {
        VStack(spacing: 0) {
            metricRow(title: "Wind", value: speedText(current.windSpeed))
            Divider().overlay(Color.white.opacity(0.16))
            metricRow(title: "Gusts", value: speedText(gustSpeed))
            Divider().overlay(Color.white.opacity(0.16))
            metricRow(title: "Direction", value: directionText(current.windDirection))
        }
        .frame(maxWidth: .infinity)
    }

    private func metricRow(title: String, value: String) -> some View {
        HStack {
            Text(title)
                .font(.system(size: 18, weight: .medium))
                .foregroundStyle(.white)
            Spacer()
            Text(value)
                .font(.system(size: 18, weight: .medium))
                .foregroundStyle(.white.opacity(0.45))
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 13)
    }

    private var dial: some View {
        ZStack {
            tickRing
            directionVector
                .zIndex(0)
            centerPlate
                .zIndex(1)

            VStack(spacing: -2) {
                Text(speedNumber(current.windSpeed))
                    .font(.system(size: 22, weight: .light))
                    .foregroundStyle(.white)
                Text("m/s")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(.white)
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
                    .stroke(Color.white.opacity(0.06), lineWidth: 1)
            )
    }

    private var tickRing: some View {
        ZStack {
            ForEach(0..<72, id: \.self) { index in
                let major = index % 6 == 0
                RoundedRectangle(cornerRadius: 1)
                    .fill(Color.white.opacity(major ? 0.22 : 0.12))
                    .frame(width: major ? 1.5 : 1, height: major ? 10 : 6)
                    .offset(y: major ? -62 : -60)
                    .rotationEffect(.degrees(Double(index) * 5))
            }
        }
    }

    private var directionVector: some View {
        // FMI direction is where wind comes FROM (0=N, 90=E).
        // The arrow should point where wind goes TO, mapped to screen rotation.
        let angle = -((current.windDirection ?? 0) + 90)
        return ZStack {
            Capsule()
                .fill(Color.white)
                .frame(width: 28, height: 3)
                .offset(x: -46)
            Capsule()
                .fill(Color.white)
                .frame(width: 34, height: 3)
                .offset(x: 32)
            Circle()
                .fill(Color.white)
                .frame(width: 14, height: 14)
                .offset(x: -52)
            Image(systemName: "arrowtriangle.right.fill")
                .font(.system(size: 14))
                .foregroundStyle(.white)
                .offset(x: 50)
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
            .foregroundStyle(.white.opacity(0.8))
            .offset(x: x, y: y)
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
                windDirection: 266.0,
                humidity: 84.0,
                pressure: 1012.0,
                observedAt: .now
            ),
            gustSpeed: 2.0
        )
        .padding()
    }
}
