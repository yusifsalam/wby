import CoreLocation
import SwiftUI

struct LeaderboardView: View {
    let locationService: LocationService
    let weatherService: WeatherService

    @Environment(\.dismiss) private var dismiss
    @State private var response: LeaderboardResponse?
    @State private var isLoading = false
    @State private var errorMessage: String?

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
            .refreshable { await fetchLeaderboard() }

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
                }
                .padding()
                Spacer()
            }
        }
        .task { await fetchLeaderboard() }
    }

    private func leaderboardCard(_ entry: LeaderboardEntry) -> some View {
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
        .weatherCard()
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
        return "\(hours)h ago"
    }

    private func fetchLeaderboard() async {
        guard let coord = locationService.coordinate else { return }
        isLoading = response == nil
        defer { isLoading = false }
        do {
            response = try await weatherService.fetchLeaderboard(lat: coord.latitude, lon: coord.longitude)
            errorMessage = nil
        } catch {
            if response == nil {
                errorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
            }
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
            weatherService: WeatherService()
        )
    }
}
