import CoreLocation
import SwiftUI
import UIKit

struct WeatherMapView: View {
    let locationService: LocationService
    let favoritesStore: FavoritesStore
    private let disableAutoLoad: Bool
    private let previewConfig: PreviewConfig?

    @Environment(\.dismiss) private var dismiss
    @AppStorage(MapLayerKind.storageKey) private var layer: MapLayerKind = .temperature
    @AppStorage(MapRampStyle.storageKey) private var mapRamp: MapRampStyle = .smooth
    @State private var viewModel: WeatherMapViewModel
    @State private var safeAreaBottom: CGFloat = 0
    @State private var scrubberTop: CGFloat = 0
    @Namespace private var glassNamespace

    init(
        locationService: LocationService,
        favoritesStore: FavoritesStore,
        weatherStore: WeatherStore,
        disableAutoLoad: Bool = false,
        previewConfig: PreviewConfig? = nil
    ) {
        self.locationService = locationService
        self.favoritesStore = favoritesStore
        self.disableAutoLoad = disableAutoLoad
        self.previewConfig = previewConfig
        _viewModel = State(
            wrappedValue: WeatherMapViewModel(
                weatherStore: weatherStore,
                networkEnabled: !disableAutoLoad,
                initialMeta: previewConfig?.overlayMeta,
                initialFavoriteWeather: previewConfig?.favoriteWeatherByID ?? [:],
                initialOverlaySeed: previewConfig?.overlaySeed
            )
        )
    }

    var body: some View {
        ZStack {
            WeatherMapUIKitBridge(viewModel: viewModel, bottomInset: max(0, safeAreaBottom - scrubberTop + 6))
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

            GlassEffectContainer(spacing: 4) {
                VStack {
                    HStack(alignment: .top) {
                        VStack(alignment: .leading, spacing: 10) {
                            Button("Close map", systemImage: "xmark", role: .close) {
                                dismiss()
                            }
                            .labelStyle(.iconOnly)
                            .font(.system(size: 17, weight: .medium))
                            .buttonStyle(.glass)
                            .buttonBorderShape(.circle)
                            .controlSize(.large)
                            .tint(.primary)

                            if viewModel.selectedLayer == .temperature {
                                TemperatureLegendView()
                                    .glassEffectID("legend", in: glassNamespace)
                            } else {
                                PrecipitationLegendView(ramp: mapRamp)
                                    .glassEffectID("legend", in: glassNamespace)
                            }
                        }
                        Spacer()
                        VStack(alignment: .trailing, spacing: 10) {
                            layerMenu
                            if let min = viewModel.meta?.minTemp, let max = viewModel.meta?.maxTemp {
                                Text("\(min.displayText())° ... \(max.displayText())°")
                                    .font(.caption2.bold())
                                    .padding(10)
                                    .glassEffect(in: RoundedRectangle(cornerRadius: 10))
                            }
                        }
                    }
                    Spacer()
                    TimeScrubberView(viewModel: viewModel)
                        .onGeometryChange(for: CGFloat.self) { $0.frame(in: .global).minY } action: { scrubberTop = $0 }
                        .glassEffectID("scrubber", in: glassNamespace)
                        .padding(.top, 8)
                }
                .padding()
            }
        }
        .onGeometryChange(for: CGFloat.self) { $0.frame(in: .global).maxY } action: { safeAreaBottom = $0 }
        .task {
            applyLayerFromSettings()
            viewModel.setPreferredCenter(locationService.coordinate)
            viewModel.setFavoriteLocations(favoritesStore.favorites)
            if !disableAutoLoad {
                _ = await locationService.requestFreshLocation()
            }
        }
        .onChange(of: layer) {
            applyLayerFromSettings()
        }
        .onChange(of: locationService.coordinate.map { "\($0.latitude),\($0.longitude)" }) {
            viewModel.setPreferredCenter(locationService.coordinate)
        }
        .onChange(of: favoritesStore.favorites) {
            viewModel.setFavoriteLocations(favoritesStore.favorites)
        }
    }

    private func applyLayerFromSettings() {
        withAnimation {
            viewModel.setSelectedLayer(layer)
        }
        viewModel.prefetchTimeline()
    }

    private var layerMenu: some View {
        Menu {
            Picker("Layer", selection: $layer) {
                ForEach(MapLayerKind.allCases, id: \.self) { layer in
                    Label(layer.displayName, systemImage: layer.symbolName)
                        .tag(layer)
                }
            }
        } label: {
            Label("Map layer", systemImage: "square.3.layers.3d")
                .font(.system(size: 17, weight: .medium))
        }
        .labelStyle(.iconOnly)
        .menuStyle(.button)
        .buttonStyle(.glass)
        .buttonBorderShape(.circle)
        .controlSize(.large)
        .tint(.primary)
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
    WeatherMapView(
        locationService: WeatherMapPreviewFixture.makeLocationService(),
        favoritesStore: WeatherMapPreviewFixture.makeStore(),
        weatherStore: WeatherStore(),
        disableAutoLoad: true,
        previewConfig: WeatherMapPreviewFixture.previewConfig
    )
}

private enum WeatherMapPreviewFixture {
    static func makeStore() -> FavoritesStore {
        FavoritesStore(
            initialFavorites: FavoriteLocation.weatherMapPreviewFavorites,
            persistenceEnabled: false
        )
    }

    @MainActor
    static func makeLocationService() -> LocationService {
        let locationService = LocationService()
        locationService.coordinate = CLLocationCoordinate2D(
            latitude: FavoriteLocation.previewHelsinki.latitude,
            longitude: FavoriteLocation.previewHelsinki.longitude
        )
        return locationService
    }

    static let previewConfig = WeatherMapView.PreviewConfig(
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
