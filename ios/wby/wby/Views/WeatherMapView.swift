import CoreLocation
import SwiftUI
import UIKit

struct WeatherMapView: View {
    let locationService: LocationService
    let favoritesStore: FavoritesStore
    private let overlayTimeZone = TimeZone(identifier: "Europe/Helsinki")!
    private let disableAutoLoad: Bool
    private let previewConfig: PreviewConfig?

    @Environment(\.dismiss) private var dismiss
    @StateObject private var viewModel: WeatherMapViewModel

    init(
        locationService: LocationService,
        favoritesStore: FavoritesStore,
        weatherService: WeatherService,
        disableAutoLoad: Bool = false,
        previewConfig: PreviewConfig? = nil
    ) {
        self.locationService = locationService
        self.favoritesStore = favoritesStore
        self.disableAutoLoad = disableAutoLoad
        self.previewConfig = previewConfig
        _viewModel = StateObject(
            wrappedValue: WeatherMapViewModel(
                overlayService: MapOverlayService(weatherService: weatherService),
                weatherService: weatherService,
                networkEnabled: !disableAutoLoad,
                initialMeta: previewConfig?.overlayMeta,
                initialFavoriteWeather: previewConfig?.favoriteWeatherByID ?? [:],
                initialOverlaySeed: previewConfig?.overlaySeed
            )
        )
    }

    var body: some View {
        ZStack {
            WeatherMapUIKitBridge(viewModel: viewModel)
                .ignoresSafeArea()

            if let canvasOverlay = previewConfig?.canvasOverlay {
                LinearGradient(
                    colors: canvasOverlay.colors,
                    startPoint: .top,
                    endPoint: .bottom
                )
                .opacity(canvasOverlay.opacity)
                .ignoresSafeArea()
                .allowsHitTesting(false)
            }

            VStack {
                HStack(alignment: .top) {
                    VStack(alignment: .leading, spacing: 10) {
                        Button {
                            dismiss()
                        } label: {
                            Image(systemName: "xmark")
                                .font(.system(size: 14, weight: .bold))
                                .frame(width: 36, height: 36)
                                .background(.ultraThinMaterial, in: Circle())
                        }
                        .buttonStyle(.plain)
                        .accessibilityLabel("Close map")

                        TemperatureLegendView()

                        Button {
                            viewModel.setOverlayMode(viewModel.overlayMode.toggled)
                        } label: {
                            Text(viewModel.overlayMode.displayName)
                                .font(.caption.bold())
                                .padding(.horizontal, 12)
                                .padding(.vertical, 6)
                                .background(.ultraThinMaterial, in: Capsule())
                        }
                        .buttonStyle(.plain)
                        .accessibilityLabel("Toggle overlay renderer")
                    }
                    Spacer()
                    if let meta = viewModel.meta {
                        VStack(alignment: .trailing, spacing: 4) {
                            if let min = meta.minTemp, let max = meta.maxTemp {
                                Text("\(Int(min.rounded()))° ... \(Int(max.rounded()))°")
                                    .font(.caption2.bold())
                            }
                            if let dataTime = meta.dataTime {
                                Text(formatOverlayDataTime(dataTime))
                                    .font(.caption2)
                                    .foregroundStyle(.secondary)
                            }
                        }
                        .padding(10)
                        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 10))
                    }
                }
                Spacer()
            }
            .padding()
        }
        .task {
            viewModel.setPreferredCenter(locationService.coordinate)
            viewModel.setFavoriteLocations(favoritesStore.favorites)
            if !disableAutoLoad {
                _ = await locationService.requestFreshLocation()
            }
        }
        .onChange(of: locationService.coordinate.map { "\($0.latitude),\($0.longitude)" }) {
            viewModel.setPreferredCenter(locationService.coordinate)
        }
        .onChange(of: favoritesStore.favorites) {
            viewModel.setFavoriteLocations(favoritesStore.favorites)
        }
    }

    private func formatOverlayDataTime(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.dateStyle = .none
        formatter.timeStyle = .short
        formatter.timeZone = overlayTimeZone
        return formatter.string(from: date)
    }

    struct PreviewConfig {
        struct CanvasOverlay {
            let colors: [Color]
            let opacity: Double
        }

        let overlayMeta: OverlayMeta?
        let favoriteWeatherByID: [UUID: FavoritePinWeather]
        let overlaySeed: OverlaySeed?
        let canvasOverlay: CanvasOverlay?
    }
}

#Preview("Weather Map - Populated") {
    let favorites = FavoriteLocation.weatherMapPreviewFavorites

    let store = FavoritesStore(
        initialFavorites: favorites,
        persistenceEnabled: false
    )

    let locationService = LocationService()
    locationService.coordinate = CLLocationCoordinate2D(
        latitude: FavoriteLocation.previewHelsinki.latitude,
        longitude: FavoriteLocation.previewHelsinki.longitude
    )

    return WeatherMapView(
        locationService: locationService,
        favoritesStore: store,
        weatherService: WeatherService(),
        disableAutoLoad: true,
        previewConfig: .init(
            overlayMeta: OverlayMeta(
                dataTime: Date.now.addingTimeInterval(-10 * 60),
                minTemp: -8.0,
                maxTemp: 13.0
            ),
            favoriteWeatherByID: [
                FavoriteLocation.previewHelsinki.id: FavoritePinWeather(current: 8, low: 4, high: 11),
                FavoriteLocation.previewTampere.id: FavoritePinWeather(current: 5, low: 1, high: 8),
                FavoriteLocation.previewTurku.id: FavoritePinWeather(current: 7, low: 3, high: 10),
            ],
            overlaySeed: WeatherMapPreviewAssets.overlaySeed(),
            canvasOverlay: .init(
                colors: [
                    Color(red: 121.0 / 255.0, green: 45.0 / 255.0, blue: 199.0 / 255.0),
                    Color(red: 96.0 / 255.0, green: 191.0 / 255.0, blue: 255.0 / 255.0),
                    Color(red: 116.0 / 255.0, green: 199.0 / 255.0, blue: 85.0 / 255.0),
                    Color(red: 235.0 / 255.0, green: 168.0 / 255.0, blue: 58.0 / 255.0),
                ],
                opacity: 0.23
            )
        )
    )
}

private enum WeatherMapPreviewAssets {
    static func overlaySeed() -> OverlaySeed {
        let size = CGSize(width: 512, height: 512)
        let renderer = UIGraphicsImageRenderer(size: size)
        let image = renderer.image { context in
            let cgContext = context.cgContext
            let colors = [
                UIColor(red: 0.33, green: 0.17, blue: 0.70, alpha: 0.82).cgColor,
                UIColor(red: 0.23, green: 0.45, blue: 0.86, alpha: 0.82).cgColor,
                UIColor(red: 0.18, green: 0.68, blue: 0.87, alpha: 0.82).cgColor,
                UIColor(red: 0.34, green: 0.77, blue: 0.37, alpha: 0.82).cgColor,
                UIColor(red: 0.95, green: 0.66, blue: 0.26, alpha: 0.82).cgColor,
                UIColor(red: 0.79, green: 0.20, blue: 0.21, alpha: 0.82).cgColor,
            ] as CFArray

            let colorSpace = CGColorSpaceCreateDeviceRGB()
            let locations: [CGFloat] = [0.0, 0.2, 0.4, 0.62, 0.82, 1.0]
            guard let gradient = CGGradient(
                colorsSpace: colorSpace,
                colors: colors,
                locations: locations
            ) else { return }

            cgContext.drawLinearGradient(
                gradient,
                start: CGPoint(x: size.width / 2, y: 0),
                end: CGPoint(x: size.width / 2, y: size.height),
                options: []
            )
        }
        return OverlaySeed(image: image, bbox: .finland)
    }
}
