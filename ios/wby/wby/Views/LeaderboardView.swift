import CoreLocation
import MapKit
import SwiftUI

enum LeaderboardTimeframe: String, CaseIterable {
    case now = "now"
    case oneHour = "1h"
    case oneDay = "24h"
    case threeDays = "3d"
    case sevenDays = "7d"

    var label: String {
        switch self {
        case .now: "Now"
        case .oneHour: "1h"
        case .oneDay: "24h"
        case .threeDays: "3d"
        case .sevenDays: "7d"
        }
    }
}

struct LeaderboardView: View {
    let locationService: LocationService
    let weatherService: WeatherService
    private let initialResponse: LeaderboardResponse?
    private let disableAutoLoad: Bool

    @Environment(\.dismiss) private var dismiss
    @State private var response: LeaderboardResponse?
    @State private var isLoading = false
    @State private var errorMessage: String?
    @State private var timeframe: LeaderboardTimeframe = .now

    init(
        locationService: LocationService,
        weatherService: WeatherService,
        initialResponse: LeaderboardResponse? = nil,
        disableAutoLoad: Bool = false
    ) {
        self.locationService = locationService
        self.weatherService = weatherService
        self.initialResponse = initialResponse
        self.disableAutoLoad = disableAutoLoad
        _response = State(initialValue: initialResponse)
    }

    var body: some View {
        ZStack {
            LinearGradient(
                colors: WeatherScene.clearDay.gradientColors,
                startPoint: .top,
                endPoint: .bottom
            )
            .ignoresSafeArea()

            ScrollView {
                VStack(spacing: 12) {
                    if let response, !response.leaderboard.isEmpty {
                        ForEach(response.leaderboard) { entry in
                            leaderboardCard(entry)
                        }
                    } else if isLoading {
                        ProgressView("Loading leaderboard...")
                            .tint(.primary)
                            .foregroundStyle(.primary)
                            .padding(.top, 100)
                    } else {
                        ContentUnavailableView(
                            "No Leaderboard Data",
                            systemImage: "chart.bar",
                            description: Text(errorMessage ?? "Pull down to refresh")
                        )
                    }
                }
                .padding()
                .padding(.top, 44)
            }
            .scrollBounceBehavior(.always)
            .refreshable {
                guard !disableAutoLoad else { return }
                await fetchLeaderboard()
            }

            VStack {
                HStack {
                    Button { dismiss() } label: {
                        Image(systemName: "xmark")
                            .font(.system(size: 14, weight: .bold))
                            .frame(width: 36, height: 36)
                            .background(.ultraThinMaterial, in: Circle())
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Close")

                    Spacer()

                    Picker("Timeframe", selection: $timeframe) {
                        ForEach(LeaderboardTimeframe.allCases, id: \.self) { tf in
                            Text(tf.label).tag(tf)
                        }
                    }
                    .pickerStyle(.segmented)
                    .frame(width: 200)
                }
                .padding()
                Spacer()
            }
        }
        .task {
            guard !disableAutoLoad else { return }
            await fetchLeaderboard()
        }
        .onChange(of: timeframe) {
            guard !disableAutoLoad else { return }
            Task { await fetchLeaderboard() }
        }
    }

    private func leaderboardCard(_ entry: LeaderboardEntry) -> some View {
        HStack(spacing: 0) {
            VStack(alignment: .leading, spacing: 12) {
                Label(cardTitle(for: entry.type), systemImage: cardIcon(for: entry.type))
                    .font(.caption)
                    .foregroundStyle(.secondary)

                Text(entry.stationName)
                    .font(.title2.weight(.medium))
                    .foregroundStyle(.primary)

                HStack(alignment: .firstTextBaseline, spacing: 4) {
                    Text(formattedValue(entry.value, type: entry.type))
                        .font(.system(size: 48, weight: .light))
                        .foregroundStyle(.primary)
                    Text(entry.unit)
                        .font(.title3)
                        .foregroundStyle(.secondary)
                }

                Text(subtitle(for: entry))
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }

            if let lat = entry.lat, let lon = entry.lon {
                Spacer(minLength: 12)
                stationMap(lat: lat, lon: lon)
            }
        }
        .frame(height: 180)
        .weatherCard()
    }

    private func stationMap(lat: Double, lon: Double) -> some View {
        StationMapSnapshot(lat: lat, lon: lon)
            .frame(width: 150, height: 180)
            .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
    }

    private func cardTitle(for type: String) -> String {
        switch type {
        case "coldest": return "COLDEST IN FINLAND"
        case "warmest": return "WARMEST IN FINLAND"
        case "windiest": return "WINDIEST IN FINLAND"
        default: return type.uppercased()
        }
    }

    private func cardIcon(for type: String) -> String {
        switch type {
        case "coldest": return "snowflake"
        case "warmest": return "sun.max.fill"
        case "windiest": return "wind"
        default: return "chart.bar"
        }
    }

    private func formattedValue(_ value: Double, type: String) -> String {
        switch type {
        case "coldest", "warmest":
            return String(format: "%.1f°", value)
        case "windiest":
            return String(format: "%.1f", value)
        default:
            return String(format: "%.1f", value)
        }
    }

    private func subtitle(for entry: LeaderboardEntry) -> String {
        let distText = entry.distanceKm < 1
            ? "<1 km away"
            : "\(Int(entry.distanceKm.rounded())) km away"
        let agoText = relativeTime(entry.observedAt)
        return "\(distText) \u{00B7} \(agoText)"
    }

    private func relativeTime(_ date: Date) -> String {
        let seconds = Int(-date.timeIntervalSinceNow)
        if seconds < 60 { return "just now" }
        let minutes = seconds / 60
        if minutes < 60 { return "\(minutes) min ago" }
        let hours = minutes / 60
        if hours < 24 { return "\(hours)h ago" }
        let days = hours / 24
        return "\(days)d ago"
    }

    private func fetchLeaderboard() async {
        guard let coord = locationService.coordinate else { return }
        isLoading = response == nil
        defer { isLoading = false }
        do {
            response = try await weatherService.fetchLeaderboard(lat: coord.latitude, lon: coord.longitude, timeframe: timeframe.rawValue)
            errorMessage = nil
        } catch {
            if response == nil {
                errorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
            }
        }
    }
}

private struct StationMapSnapshot: View {
    let lat: Double
    let lon: Double

    @State private var image: UIImage?

    var body: some View {
        Group {
            if let image {
                Image(uiImage: image)
                    .resizable()
                    .scaledToFill()
            } else {
                Rectangle()
                    .fill(.quaternary)
            }
        }
        .task(id: "\(lat),\(lon)") {
            image = await Self.snapshotCache.snapshot(lat: lat, lon: lon)
        }
    }

    @MainActor
    private static let snapshotCache = MapSnapshotCache()
}

@MainActor
private final class MapSnapshotCache {
    private var cache: [String: UIImage] = [:]

    func snapshot(lat: Double, lon: Double) async -> UIImage? {
        let key = "\(lat),\(lon)"
        if let cached = cache[key] { return cached }

        let options = MKMapSnapshotter.Options()
        options.region = MKCoordinateRegion(
            center: CLLocationCoordinate2D(latitude: lat, longitude: lon),
            latitudinalMeters: 800_000,
            longitudinalMeters: 800_000
        )
        options.size = CGSize(width: 300, height: 360)
        options.pointOfInterestFilter = .excludingAll

        do {
            let snapshot = try await MKMapSnapshotter(options: options).start()
            let pin = snapshot.point(for: CLLocationCoordinate2D(latitude: lat, longitude: lon))

            let renderer = UIGraphicsImageRenderer(size: options.size)
            let image = renderer.image { ctx in
                snapshot.image.draw(at: .zero)
                let dotSize: CGFloat = 16
                let dotRect = CGRect(
                    x: pin.x - dotSize / 2,
                    y: pin.y - dotSize / 2,
                    width: dotSize,
                    height: dotSize
                )
                ctx.cgContext.setFillColor(UIColor.systemRed.cgColor)
                ctx.cgContext.fillEllipse(in: dotRect)
                ctx.cgContext.setStrokeColor(UIColor.white.cgColor)
                ctx.cgContext.setLineWidth(3)
                ctx.cgContext.strokeEllipse(in: dotRect)
            }
            cache[key] = image
            return image
        } catch {
            return nil
        }
    }
}

#Preview {
    ZStack {
        LinearGradient(
            colors: [.blue.opacity(0.6), .blue.opacity(0.3)],
            startPoint: .top,
            endPoint: .bottom
        )
        .ignoresSafeArea()

        LeaderboardView(
            locationService: LocationService(),
            weatherService: WeatherService(),
            initialResponse: LeaderboardResponse(
                timeframe: "now",
                leaderboard: [
                    LeaderboardEntry(type: "coldest", stationName: "Enontekiö Kilpisjärvi", lat: 69.05, lon: 20.79, value: -12.3, unit: "°C", distanceKm: 980, observedAt: Date(timeIntervalSinceNow: -600)),
                    LeaderboardEntry(type: "warmest", stationName: "Helsinki Kaisaniemi", lat: 60.18, lon: 24.94, value: 8.1, unit: "°C", distanceKm: 12, observedAt: Date(timeIntervalSinceNow: -300)),
                    LeaderboardEntry(type: "windiest", stationName: "Utö", lat: 59.78, lon: 21.37, value: 18.4, unit: "m/s", distanceKm: 195, observedAt: Date(timeIntervalSinceNow: -120)),
                ]
            ),
            disableAutoLoad: true
        )
    }
}
