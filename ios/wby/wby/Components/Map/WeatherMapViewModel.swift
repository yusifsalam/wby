import Combine
import CoreLocation
import MapKit
import UIKit

struct OverlayMeta: Equatable {
    let dataTime: Date?
    let minTemp: Double?
    let maxTemp: Double?
}

enum TemperatureText {
    static func rounded(_ value: Double?) -> Int? {
        value.map { Int($0.rounded()) }
    }

    static func value(_ value: Int?) -> String {
        value.map { "\($0)°" } ?? "--°"
    }

    static func range(low: Int?, high: Int?) -> String {
        "L \(value(low))  H \(value(high))"
    }
}

struct FavoritePinWeather: Equatable {
    let current: Int?
    let low: Int?
    let high: Int?

    static func from(response: WeatherResponse) -> FavoritePinWeather {
        let today = response.dailyForecast.first
        return FavoritePinWeather(
            current: TemperatureText.rounded(response.current.resolvedTemperature ?? response.hourlyForecast.first?.temperature),
            low: TemperatureText.rounded(today?.low),
            high: TemperatureText.rounded(today?.high)
        )
    }
}

private struct CachedFavoritePinWeather {
    let weather: FavoritePinWeather
    let fetchedAt: Date
}

enum PreviewLoadState {
    case loading
    case loaded
    case failed
}

struct PreviewWeather: Equatable {
    let current: Int?
    let conditionText: String?
    let symbolCode: String?
    let low: Int?
    let high: Int?

    static func from(response: WeatherResponse) -> PreviewWeather {
        let today = response.dailyForecast.first
        return PreviewWeather(
            current: TemperatureText.rounded(response.current.resolvedTemperature ?? response.hourlyForecast.first?.temperature),
            conditionText: WeatherSymbols.conditionDescription(from: response),
            symbolCode: WeatherSymbols.primarySymbol(from: response),
            low: TemperatureText.rounded(today?.low),
            high: TemperatureText.rounded(today?.high)
        )
    }
}

struct OverlaySeed {
    let image: UIImage
    let bbox: MapBBox
}

@MainActor
final class WeatherMapViewModel: ObservableObject {
    @Published var meta: OverlayMeta?
    @Published var overlayMode: OverlayMode

    private let favoriteWeatherTTL: TimeInterval = 10 * 60
    private let favoritePinsMaxLatitudeDelta: CLLocationDegrees = 5.8
    private let overlayRefreshInterval: TimeInterval = 3 * 60
    private let overlaySize: Int = 512
    private let overlayService: MapOverlayService
    private let weatherService: WeatherService
    private let networkEnabled: Bool
    private let initialOverlaySeed: OverlaySeed?
    private let initialSamples: TemperatureSamplesResponse?
    private let metalRenderer: TemperatureMetalRenderer?
    private weak var mapView: MKMapView?
    private var overlayTask: Task<Void, Never>?
    private var overlayLastFetchedAt: Date?
    private var samplesTask: Task<Void, Never>?
    private var samplesLastFetchedAt: Date?
    private var cachedSamples: TemperatureSamplesResponse?
    private var cachedOverlayImage: TemperatureOverlayImage?
    private var cachedMetalOverlaySeed: OverlaySeed?
    private var favoriteWeatherCache: [UUID: CachedFavoritePinWeather] = [:]
    private var favoriteWeatherTasks: [UUID: Task<Void, Never>] = [:]
    private var previewAnnotation: PreviewPinAnnotation?
    private var previewWeatherTask: Task<Void, Never>?
    private var previewGeocodeTask: Task<Void, Never>?
    private let previewHaptic = UIImpactFeedbackGenerator(style: .light)
    private var favoriteLocations: [FavoriteLocation] = []
    private var preferredCenter: CLLocationCoordinate2D?
    private var didCenterOnPreferredLocation = false

    init(
        overlayService: MapOverlayService,
        weatherService: WeatherService,
        networkEnabled: Bool = true,
        initialMeta: OverlayMeta? = nil,
        initialFavoriteWeather: [UUID: FavoritePinWeather] = [:],
        initialOverlaySeed: OverlaySeed? = nil,
        initialSamples: TemperatureSamplesResponse? = nil,
        overlayMode: OverlayMode = OverlayMode.load()
    ) {
        self.overlayService = overlayService
        self.weatherService = weatherService
        self.networkEnabled = networkEnabled
        self.initialOverlaySeed = initialOverlaySeed
        self.initialSamples = initialSamples
        self.metalRenderer = TemperatureMetalRenderer()
        self.meta = initialMeta
        let resolvedMode: OverlayMode
        if overlayMode == .metal, metalRenderer == nil {
            resolvedMode = .png
        } else {
            resolvedMode = overlayMode
        }
        self.overlayMode = resolvedMode
        if resolvedMode != overlayMode {
            resolvedMode.save()
        }
        self.favoriteWeatherCache = initialFavoriteWeather.mapValues {
            CachedFavoritePinWeather(weather: $0, fetchedAt: Date())
        }
        if let initialSamples {
            self.cachedSamples = initialSamples
        }
    }

    deinit {
        overlayTask?.cancel()
        samplesTask?.cancel()
        favoriteWeatherTasks.values.forEach { $0.cancel() }
        previewWeatherTask?.cancel()
        previewGeocodeTask?.cancel()
    }

    func bind(mapView: MKMapView) {
        guard self.mapView !== mapView else { return }
        self.mapView = mapView
        didCenterOnPreferredLocation = false
        applyInitialRegion(on: mapView)
        if overlayMode == .png, let initialOverlaySeed {
            applyOverlay(
                image: initialOverlaySeed.image,
                bbox: initialOverlaySeed.bbox,
                on: mapView
            )
        }
        updateFavoriteAnnotations(on: mapView, favorites: favoriteLocations)
        applyOverlayModeVisibility()
        kickOffRefreshForCurrentMode()
    }

    func setOverlayMode(_ mode: OverlayMode) {
        guard mode != overlayMode else { return }
        if mode == .metal, metalRenderer == nil {
            return
        }
        overlayMode = mode
        mode.save()
        applyOverlayModeVisibility()
        if let cachedSamples, mode == .metal {
            meta = OverlayMeta(
                dataTime: cachedSamples.dataTime,
                minTemp: cachedSamples.minTemp,
                maxTemp: cachedSamples.maxTemp
            )
        } else if let cachedOverlayImage, mode == .png {
            meta = OverlayMeta(
                dataTime: cachedOverlayImage.dataTime,
                minTemp: cachedOverlayImage.minTemp,
                maxTemp: cachedOverlayImage.maxTemp
            )
        }
        kickOffRefreshForCurrentMode()
    }

    func setFavoriteLocations(_ favorites: [FavoriteLocation]) {
        favoriteLocations = favorites
        guard let mapView else { return }
        updateFavoriteAnnotations(on: mapView, favorites: favorites)
    }

    func setPreferredCenter(_ coordinate: CLLocationCoordinate2D?) {
        preferredCenter = coordinate
        guard let mapView else { return }
        guard let coordinate, !didCenterOnPreferredLocation else { return }
        mapView.setRegion(
            MKCoordinateRegion(
                center: coordinate,
                span: MKCoordinateSpan(latitudeDelta: 1.2, longitudeDelta: 1.2)
            ),
            animated: true
        )
        didCenterOnPreferredLocation = true
    }

    func handleRegionDidChange(on mapView: MKMapView) {
        applyFavoritePinVisibility(on: mapView)
        refreshVisibleFavoriteWeather(on: mapView)
        kickOffRefreshForCurrentMode()
    }

    func handleLongPress(_ recognizer: UILongPressGestureRecognizer) {
        guard recognizer.state == .began,
              let mapView = recognizer.view as? MKMapView
        else { return }

        let point = recognizer.location(in: mapView)
        let coordinate = mapView.convert(point, toCoordinateFrom: mapView)

        cancelPreviewTasks()
        removePreviewAnnotation(from: mapView)

        let annotation = PreviewPinAnnotation(coordinate: coordinate)
        annotation.placeName = formatFallbackPlaceName(coordinate)
        previewAnnotation = annotation
        mapView.addAnnotation(annotation)

        previewHaptic.impactOccurred()
        guard networkEnabled else {
            annotation.loadState = .failed
            refreshPreviewView(for: annotation, on: mapView)
            return
        }
        startPreviewWeatherFetch(for: annotation, on: mapView)
        startPreviewGeocode(for: annotation, on: mapView)
    }

    func dismissPreview(on mapView: MKMapView) {
        cancelPreviewTasks()
        removePreviewAnnotation(from: mapView)
    }

    func updateFavoriteAnnotations(on mapView: MKMapView, favorites: [FavoriteLocation]) {
        let validIDs = Set(favorites.map(\.id))
        let staleTaskIDs = favoriteWeatherTasks.keys.filter { !validIDs.contains($0) }
        for id in staleTaskIDs {
            favoriteWeatherTasks[id]?.cancel()
            favoriteWeatherTasks[id] = nil
            favoriteWeatherCache[id] = nil
        }

        let existing = mapView.favoritePinAnnotations
        if !existing.isEmpty {
            mapView.removeAnnotations(existing)
        }
        let newAnnotations = favorites.map {
            FavoritePinAnnotation(
                id: $0.id,
                title: $0.name,
                coordinate: CLLocationCoordinate2D(latitude: $0.latitude, longitude: $0.longitude),
                weather: favoriteWeatherCache[$0.id]?.weather
            )
        }
        mapView.addAnnotations(newAnnotations)
        applyFavoritePinVisibility(on: mapView)
        refreshVisibleFavoriteWeather(on: mapView)
    }

    func scheduleOverlayRefresh(on mapView: MKMapView) {
        guard networkEnabled else { return }
        if let lastFetched = overlayLastFetchedAt,
           Date().timeIntervalSince(lastFetched) < overlayRefreshInterval
        {
            return
        }

        overlayTask?.cancel()
        overlayTask = Task { [weak mapView] in
            guard !Task.isCancelled else { return }
            do {
                let overlayImage = try await overlayService.fetchTemperatureOverlay(
                    bbox: .finland,
                    width: overlaySize,
                    height: overlaySize
                )
                guard !Task.isCancelled, let mapView, let image = UIImage(data: overlayImage.imageData) else { return }
                overlayLastFetchedAt = Date()
                cachedOverlayImage = overlayImage
                if overlayMode == .png {
                    applyOverlay(
                        image: image,
                        bbox: overlayImage.bbox,
                        on: mapView
                    )
                    meta = OverlayMeta(
                        dataTime: overlayImage.dataTime,
                        minTemp: overlayImage.minTemp,
                        maxTemp: overlayImage.maxTemp
                    )
                }
            } catch {
                // Keep previous successful overlay visible on fetch errors.
            }
        }
    }

    func scheduleSamplesRefresh() {
        guard networkEnabled else { return }
        if let last = samplesLastFetchedAt,
           Date().timeIntervalSince(last) < overlayRefreshInterval
        {
            return
        }

        samplesTask?.cancel()
        samplesTask = Task { [weak self] in
            guard let self else { return }
            guard !Task.isCancelled else { return }
            do {
                let resp = try await overlayService.fetchTemperatureSamples()
                guard !Task.isCancelled else { return }
                samplesLastFetchedAt = Date()
                cachedSamples = resp
                updateMetalOverlayCache(from: resp)
                if overlayMode == .metal {
                    meta = OverlayMeta(
                        dataTime: resp.dataTime,
                        minTemp: resp.minTemp,
                        maxTemp: resp.maxTemp
                    )
                    if let seed = cachedMetalOverlaySeed, let mapView {
                        applyOverlay(image: seed.image, bbox: seed.bbox, on: mapView)
                    }
                }
            } catch {
                // Keep previous samples on fetch errors.
            }
        }
    }

    private func kickOffRefreshForCurrentMode() {
        guard let mapView else { return }
        switch overlayMode {
        case .png:
            scheduleOverlayRefresh(on: mapView)
        case .metal:
            scheduleSamplesRefresh()
        }
    }

    private func applyOverlayModeVisibility() {
        guard let mapView else { return }
        switch overlayMode {
        case .png:
            if let cachedOverlayImage,
               let image = UIImage(data: cachedOverlayImage.imageData) {
                applyOverlay(image: image, bbox: cachedOverlayImage.bbox, on: mapView)
            }
        case .metal:
            if let seed = cachedMetalOverlaySeed {
                applyOverlay(image: seed.image, bbox: seed.bbox, on: mapView)
            } else if let cachedSamples {
                updateMetalOverlayCache(from: cachedSamples)
                if let seed = cachedMetalOverlaySeed {
                    applyOverlay(image: seed.image, bbox: seed.bbox, on: mapView)
                }
            } else {
                let existing = mapView.overlays.compactMap { $0 as? TemperatureImageOverlay }
                if !existing.isEmpty {
                    mapView.removeOverlays(existing)
                }
            }
        }
    }

    private func updateMetalOverlayCache(from response: TemperatureSamplesResponse) {
        guard let renderer = metalRenderer else { return }
        renderer.setSamples(response.samples)
        guard let image = renderer.renderImage(
            bounds: MercatorBounds.finland,
            width: overlaySize,
            height: overlaySize
        ) else { return }
        cachedMetalOverlaySeed = OverlaySeed(image: image, bbox: .finland)
    }

    private func startPreviewWeatherFetch(
        for annotation: PreviewPinAnnotation,
        on mapView: MKMapView
    ) {
        previewWeatherTask = Task { [weak self, weak mapView] in
            guard let self else { return }
            do {
                let response = try await self.weatherService.fetchWeather(
                    lat: annotation.coordinate.latitude,
                    lon: annotation.coordinate.longitude
                )
                guard let mapView = activePreviewMapView(for: annotation, mapView: mapView) else { return }

                annotation.weather = PreviewWeather.from(response: response)
                annotation.loadState = .loaded
                refreshPreviewView(for: annotation, on: mapView)
            } catch WeatherError.httpStatus(404, _) {
                guard let mapView = activePreviewMapView(for: annotation, mapView: mapView) else { return }
                self.dismissPreview(on: mapView)
            } catch {
                guard let mapView = activePreviewMapView(for: annotation, mapView: mapView) else { return }
                annotation.loadState = .failed
                refreshPreviewView(for: annotation, on: mapView)
            }
        }
    }

    private func startPreviewGeocode(
        for annotation: PreviewPinAnnotation,
        on mapView: MKMapView
    ) {
        let location = CLLocation(
            latitude: annotation.coordinate.latitude,
            longitude: annotation.coordinate.longitude
        )
        previewGeocodeTask = Task { [weak self, weak mapView] in
            guard let self else { return }
            let geocoder = CLGeocoder()
            let placemarks: [CLPlacemark]
            do {
                placemarks = try await geocoder.reverseGeocodeLocation(location)
            } catch {
                placemarks = []
            }
            guard let mapView = activePreviewMapView(for: annotation, mapView: mapView) else { return }

            let resolved = placemarks.first.flatMap { placemark in
                placemark.locality
                    ?? placemark.subLocality
                    ?? placemark.administrativeArea
                    ?? placemark.country
            }
            annotation.placeName = resolved ?? formatFallbackPlaceName(annotation.coordinate)
            refreshPreviewView(for: annotation, on: mapView)
        }
    }

    private func refreshVisibleFavoriteWeather(on mapView: MKMapView) {
        guard networkEnabled else {
            cancelFavoriteWeatherTasks(except: [])
            return
        }
        if !areFavoritePinsVisible(on: mapView) {
            cancelFavoriteWeatherTasks(except: [])
            return
        }

        let visible = mapView.favoritePinAnnotations
            .filter { mapView.visibleMapRect.contains(MKMapPoint($0.coordinate)) }
        let visibleIDs = Set(visible.map(\.id))

        cancelFavoriteWeatherTasks(except: visibleIDs)

        let now = Date()
        for annotation in visible {
            if let cached = favoriteWeatherCache[annotation.id],
               now.timeIntervalSince(cached.fetchedAt) < favoriteWeatherTTL
            {
                apply(weather: cached.weather, to: annotation, on: mapView)
                continue
            }

            if favoriteWeatherTasks[annotation.id] != nil {
                continue
            }
            startVisibleFavoriteWeatherFetch(
                id: annotation.id,
                latitude: annotation.coordinate.latitude,
                longitude: annotation.coordinate.longitude
            )
        }
    }

    private func startVisibleFavoriteWeatherFetch(id: UUID, latitude: Double, longitude: Double) {
        favoriteWeatherTasks[id] = Task { [weak self] in
            guard let self else { return }
            defer { self.favoriteWeatherTasks[id] = nil }

            do {
                let response = try await weatherService.fetchWeather(lat: latitude, lon: longitude)
                guard !Task.isCancelled else { return }

                let weather = FavoritePinWeather.from(response: response)
                favoriteWeatherCache[id] = CachedFavoritePinWeather(weather: weather, fetchedAt: Date())

                guard let mapView else { return }
                guard let annotation = mapView.favoritePinAnnotation(id: id) else {
                    return
                }
                apply(weather: weather, to: annotation, on: mapView)
            } catch {
                // Keep previous cached weather on failures.
            }
        }
    }

    private func apply(weather: FavoritePinWeather, to annotation: FavoritePinAnnotation, on mapView: MKMapView) {
        annotation.weather = weather
        refreshFavoriteView(for: annotation, on: mapView)
    }

    private func areFavoritePinsVisible(on mapView: MKMapView) -> Bool {
        mapView.region.span.latitudeDelta <= favoritePinsMaxLatitudeDelta
    }

    private func applyFavoritePinVisibility(on mapView: MKMapView) {
        let visible = areFavoritePinsVisible(on: mapView)
        for annotation in mapView.favoritePinAnnotations {
            annotation.isVisibleAtCurrentZoom = visible
            refreshFavoriteView(for: annotation, on: mapView)
        }
    }

    private func cancelFavoriteWeatherTasks(except retainedIDs: Set<UUID>) {
        for id in Array(favoriteWeatherTasks.keys) where !retainedIDs.contains(id) {
            favoriteWeatherTasks[id]?.cancel()
            favoriteWeatherTasks[id] = nil
        }
    }

    private func cancelPreviewTasks() {
        previewWeatherTask?.cancel()
        previewGeocodeTask?.cancel()
        previewWeatherTask = nil
        previewGeocodeTask = nil
    }

    private func removePreviewAnnotation(from mapView: MKMapView) {
        if let previewAnnotation {
            mapView.removeAnnotation(previewAnnotation)
        }
        previewAnnotation = nil
    }

    private func activePreviewMapView(
        for annotation: PreviewPinAnnotation,
        mapView: MKMapView?
    ) -> MKMapView? {
        guard !Task.isCancelled, let mapView else { return nil }
        guard previewAnnotation === annotation else { return nil }
        return mapView
    }

    private func refreshFavoriteView(for annotation: FavoritePinAnnotation, on mapView: MKMapView) {
        mapView.bubbleView(for: annotation)?.configureFavorite(with: annotation)
    }

    private func refreshPreviewView(for annotation: PreviewPinAnnotation, on mapView: MKMapView) {
        mapView.bubbleView(for: annotation)?.configurePreview(with: annotation)
    }

    private func applyOverlay(image: UIImage, bbox: MapBBox, on mapView: MKMapView) {
        let existing = mapView.overlays.compactMap { $0 as? TemperatureImageOverlay }
        if !existing.isEmpty {
            mapView.removeOverlays(existing)
        }
        mapView.addOverlay(TemperatureImageOverlay(bbox: bbox, image: image), level: .aboveRoads)
    }

    private func applyInitialRegion(on mapView: MKMapView) {
        if let coordinate = preferredCenter {
            mapView.setRegion(
                MKCoordinateRegion(
                    center: coordinate,
                    span: MKCoordinateSpan(latitudeDelta: 1.2, longitudeDelta: 1.2)
                ),
                animated: false
            )
            didCenterOnPreferredLocation = true
        } else {
            mapView.setRegion(
                MKCoordinateRegion(
                    center: CLLocationCoordinate2D(latitude: 64.9, longitude: 25.5),
                    span: MKCoordinateSpan(latitudeDelta: 9.0, longitudeDelta: 11.0)
                ),
                animated: false
            )
            didCenterOnPreferredLocation = false
        }
    }
}
