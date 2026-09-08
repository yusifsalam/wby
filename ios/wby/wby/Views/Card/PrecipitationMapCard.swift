import CoreLocation
import SwiftUI

struct PrecipitationMapCard: View {
    let coordinate: CLLocationCoordinate2D
    let weatherService: WeatherService
    let timeZone: TimeZone
    let refreshToken: Date?
    let disableAutoLoad: Bool
    let onOpenMap: () -> Void

    @Environment(\.colorScheme) private var colorScheme
    @State private var snapshot: PrecipitationMapSnapshot?
    @State private var failed = false

    private var loadKey: String {
        String(
            format: "%.2f,%.2f,%@,%.0f",
            coordinate.latitude, coordinate.longitude,
            colorScheme == .dark ? "dark" : "light",
            refreshToken?.timeIntervalSinceReferenceDate ?? 0
        )
    }

    var body: some View {
        Button(action: onOpenMap) {
            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    Label("PRECIPITATION MAP", systemImage: "cloud.rain")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Spacer()
                    Image(systemName: "chevron.right")
                        .font(.caption)
                        .foregroundStyle(.tertiary)
                }
                mapImage
                    .frame(maxWidth: .infinity)
                    .aspectRatio(
                        PrecipitationMapSnapshotLoader.size.width / PrecipitationMapSnapshotLoader.size.height,
                        contentMode: .fit
                    )
                    .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
                HStack {
                    Text(statusText)
                    Spacer()
                    if let dataTime = snapshot?.dataTime {
                        Text("Radar \(dataTime, format: timeFormat)")
                    }
                }
                .font(.caption)
                .foregroundStyle(.secondary)
            }
            .weatherCard()
        }
        .buttonStyle(.plain)
        .task(id: loadKey) {
            guard !disableAutoLoad else { return }
            let result = await PrecipitationMapSnapshotLoader.shared.snapshot(
                for: coordinate,
                style: colorScheme == .dark ? .dark : .light,
                weatherService: weatherService
            )
            guard !Task.isCancelled else { return }
            snapshot = result
            failed = result == nil
        }
    }

    @ViewBuilder
    private var mapImage: some View {
        if let snapshot {
            Image(uiImage: snapshot.image)
                .resizable()
                .scaledToFill()
        } else {
            Rectangle()
                .fill(.quaternary)
                .overlay {
                    if failed {
                        Label("Map unavailable", systemImage: "map")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    } else {
                        ProgressView()
                            .tint(.secondary)
                    }
                }
        }
    }

    private var statusText: String {
        guard let snapshot else { return failed ? "Tap to open the map" : "Loading radar…" }
        guard let maxRate = snapshot.maxRate else { return "Radar unavailable" }
        if maxRate < 0.1 { return "No precipitation nearby" }
        let rate = maxRate < 1 ? String(format: "%.1f", maxRate) : "\(Int(maxRate.rounded()))"
        return "Up to \(rate) mm/h nearby"
    }

    private var timeFormat: Date.FormatStyle {
        Date.FormatStyle(date: .omitted, time: .shortened, timeZone: timeZone)
    }
}

#Preview {
    ZStack {
        Color.blue.opacity(0.45).ignoresSafeArea()
        PrecipitationMapCard(
            coordinate: CLLocationCoordinate2D(latitude: 60.1699, longitude: 24.9384),
            weatherService: WeatherService(),
            timeZone: TimeZone(identifier: "Europe/Helsinki")!,
            refreshToken: nil,
            disableAutoLoad: true,
            onOpenMap: {}
        )
        .padding()
    }
}
