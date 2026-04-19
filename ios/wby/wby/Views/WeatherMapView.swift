import CoreLocation
import Combine
import MapKit
import SwiftUI
import UIKit

struct WeatherMapView: View {
    let locationService: LocationService
    let favoritesStore: FavoritesStore
    private let overlayTimeZone = TimeZone(identifier: "Europe/Helsinki")!

    @Environment(\.dismiss) private var dismiss
    @StateObject private var viewModel: WeatherMapViewModel

    init(
        locationService: LocationService,
        favoritesStore: FavoritesStore,
        weatherService: WeatherService
    ) {
        self.locationService = locationService
        self.favoritesStore = favoritesStore
        _viewModel = StateObject(
            wrappedValue: WeatherMapViewModel(
                overlayService: MapOverlayService(weatherService: weatherService),
                weatherService: weatherService
            )
        )
    }

    var body: some View {
        ZStack {
            WeatherMapUIKitBridge(viewModel: viewModel)
            .ignoresSafeArea()

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
            _ = await locationService.requestFreshLocation()
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
}

private struct TemperatureLegendView: View {
    private static let colors: [Color] = [
        Color(red: 198.0 / 255.0, green: 29.0 / 255.0, blue: 33.0 / 255.0),   // 40
        Color(red: 235.0 / 255.0, green: 168.0 / 255.0, blue: 58.0 / 255.0),  // 30
        Color(red: 116.0 / 255.0, green: 199.0 / 255.0, blue: 85.0 / 255.0),  // 20
        Color(red: 86.0 / 255.0, green: 208.0 / 255.0, blue: 209.0 / 255.0),  // 10
        Color(red: 96.0 / 255.0, green: 191.0 / 255.0, blue: 255.0 / 255.0),  // 0
        Color(red: 63.0 / 255.0, green: 92.0 / 255.0, blue: 222.0 / 255.0),   // -20
        Color(red: 121.0 / 255.0, green: 45.0 / 255.0, blue: 199.0 / 255.0),  // -40
    ]

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Temperature (°C)")
                .font(.caption2.bold())
            HStack(alignment: .top, spacing: 8) {
                LinearGradient(colors: Self.colors, startPoint: .top, endPoint: .bottom)
                    .frame(width: 5, height: 120)
                    .clipShape(RoundedRectangle(cornerRadius: 3))

                VStack(alignment: .leading, spacing: 5) {
                    Text("40")
                    Text("30")
                    Text("20")
                    Text("10")
                    Text("0")
                    Text("-20")
                    Text("-40")
                }
                .font(.caption2.monospacedDigit())
                .foregroundStyle(.secondary)
            }
        }
        .padding(.vertical, 10)
        .padding(.horizontal, 10)
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 10))
    }
}

private struct OverlayMeta: Equatable {
    let dataTime: Date?
    let minTemp: Double?
    let maxTemp: Double?
}

private struct FavoritePinWeather: Equatable {
    let current: Int?
    let low: Int?
    let high: Int?

    var currentText: String {
        formatTemperature(current)
    }

    var rangeText: String {
        "L \(formatTemperature(low))  H \(formatTemperature(high))"
    }

    static func from(response: WeatherResponse) -> FavoritePinWeather {
        let today = response.dailyForecast.first
        return FavoritePinWeather(
            current: roundTemperature(response.current.resolvedTemperature ?? response.hourlyForecast.first?.temperature),
            low: roundTemperature(today?.low),
            high: roundTemperature(today?.high)
        )
    }

    private static func roundTemperature(_ value: Double?) -> Int? {
        guard let value else { return nil }
        return Int(value.rounded())
    }

    private func formatTemperature(_ value: Int?) -> String {
        guard let value else { return "--°" }
        return "\(value)°"
    }
}

private struct CachedFavoritePinWeather {
    let weather: FavoritePinWeather
    let fetchedAt: Date
}

@MainActor
private final class WeatherMapViewModel: ObservableObject {
    @Published var meta: OverlayMeta?

    private let favoriteWeatherTTL: TimeInterval = 10 * 60
    private let favoritePinsMaxLatitudeDelta: CLLocationDegrees = 5.8
    private let overlayRefreshInterval: TimeInterval = 3 * 60
    private let overlaySize: Int = 512
    private let overlayService: MapOverlayService
    private let weatherService: WeatherService
    private weak var mapView: MKMapView?
    private var overlayTask: Task<Void, Never>?
    private var overlayLastFetchedAt: Date?
    private var favoriteWeatherCache: [UUID: CachedFavoritePinWeather] = [:]
    private var favoriteWeatherTasks: [UUID: Task<Void, Never>] = [:]
    private var previewAnnotation: PreviewPinAnnotation?
    private var previewWeatherTask: Task<Void, Never>?
    private var previewGeocodeTask: Task<Void, Never>?
    private let previewHaptic = UIImpactFeedbackGenerator(style: .light)
    private var favoriteLocations: [FavoriteLocation] = []
    private var preferredCenter: CLLocationCoordinate2D?
    private var didCenterOnPreferredLocation = false

    init(overlayService: MapOverlayService, weatherService: WeatherService) {
        self.overlayService = overlayService
        self.weatherService = weatherService
    }

    deinit {
        overlayTask?.cancel()
        for task in favoriteWeatherTasks.values {
            task.cancel()
        }
        previewWeatherTask?.cancel()
        previewGeocodeTask?.cancel()
    }

    func bind(mapView: MKMapView) {
        guard self.mapView !== mapView else { return }
        self.mapView = mapView
        didCenterOnPreferredLocation = false
        applyInitialRegion(on: mapView)
        updateFavoriteAnnotations(on: mapView, favorites: favoriteLocations)
        scheduleOverlayRefresh(on: mapView)
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
        scheduleOverlayRefresh(on: mapView)
    }

    func handleLongPress(_ recognizer: UILongPressGestureRecognizer) {
        guard recognizer.state == .began,
              let mapView = recognizer.view as? MKMapView
        else { return }

        let point = recognizer.location(in: mapView)
        let coordinate = mapView.convert(point, toCoordinateFrom: mapView)

        previewWeatherTask?.cancel()
        previewGeocodeTask?.cancel()
        previewWeatherTask = nil
        previewGeocodeTask = nil

        if let existing = previewAnnotation {
            mapView.removeAnnotation(existing)
        }

        let annotation = PreviewPinAnnotation(coordinate: coordinate)
        annotation.placeName = formatFallbackPlaceName(coordinate)
        previewAnnotation = annotation
        mapView.addAnnotation(annotation)

        previewHaptic.impactOccurred()
        startPreviewWeatherFetch(for: annotation, on: mapView)
        startPreviewGeocode(for: annotation, on: mapView)
    }

    func dismissPreview(on mapView: MKMapView) {
        previewWeatherTask?.cancel()
        previewGeocodeTask?.cancel()
        previewWeatherTask = nil
        previewGeocodeTask = nil
        if let annotation = previewAnnotation {
            mapView.removeAnnotation(annotation)
        }
        previewAnnotation = nil
    }

    func updateFavoriteAnnotations(on mapView: MKMapView, favorites: [FavoriteLocation]) {
        let validIDs = Set(favorites.map(\.id))
        let staleTaskIDs = favoriteWeatherTasks.keys.filter { !validIDs.contains($0) }
        for id in staleTaskIDs {
            favoriteWeatherTasks[id]?.cancel()
            favoriteWeatherTasks[id] = nil
            favoriteWeatherCache[id] = nil
        }

        let existing = mapView.annotations.compactMap { $0 as? FavoritePinAnnotation }
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
        if let lastFetched = overlayLastFetchedAt,
           Date().timeIntervalSince(lastFetched) < overlayRefreshInterval
        {
            return
        }

        overlayTask?.cancel()
        let bbox = MapBBox.finland
        let width = overlaySize
        let height = overlaySize

        overlayTask = Task { [weak mapView] in
            guard !Task.isCancelled else { return }
            do {
                let overlayImage = try await overlayService.fetchTemperatureOverlay(
                    bbox: bbox,
                    width: width,
                    height: height
                )
                guard !Task.isCancelled, let mapView, let image = UIImage(data: overlayImage.imageData) else { return }
                overlayLastFetchedAt = Date()
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
            } catch {
                // Keep previous successful overlay visible on fetch errors.
            }
        }
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
                guard !Task.isCancelled, let mapView else { return }
                guard self.previewAnnotation === annotation else { return }

                annotation.weather = PreviewWeather.from(response: response)
                annotation.loadState = .loaded
                if let view = mapView.view(for: annotation) as? BubbleAnnotationView {
                    view.configurePreview(with: annotation)
                }
            } catch WeatherError.httpStatus(404, _) {
                guard !Task.isCancelled, let mapView else { return }
                guard self.previewAnnotation === annotation else { return }
                self.dismissPreview(on: mapView)
            } catch {
                guard !Task.isCancelled, let mapView else { return }
                guard self.previewAnnotation === annotation else { return }
                annotation.loadState = .failed
                if let view = mapView.view(for: annotation) as? BubbleAnnotationView {
                    view.configurePreview(with: annotation)
                }
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
            guard !Task.isCancelled, let mapView else { return }
            guard self.previewAnnotation === annotation else { return }

            let resolved = placemarks.first.flatMap { placemark in
                placemark.locality
                    ?? placemark.subLocality
                    ?? placemark.administrativeArea
                    ?? placemark.country
            }
            annotation.placeName = resolved ?? formatFallbackPlaceName(annotation.coordinate)
            if let view = mapView.view(for: annotation) as? BubbleAnnotationView {
                view.configurePreview(with: annotation)
            }
        }
    }

    private func refreshVisibleFavoriteWeather(on mapView: MKMapView) {
        if !areFavoritePinsVisible(on: mapView) {
            let activeIDs = Array(favoriteWeatherTasks.keys)
            for id in activeIDs {
                favoriteWeatherTasks[id]?.cancel()
                favoriteWeatherTasks[id] = nil
            }
            return
        }

        let visible = mapView.annotations
            .compactMap { $0 as? FavoritePinAnnotation }
            .filter { mapView.visibleMapRect.contains(MKMapPoint($0.coordinate)) }
        let visibleIDs = Set(visible.map(\.id))

        let offscreenTaskIDs = favoriteWeatherTasks.keys.filter { !visibleIDs.contains($0) }
        for id in offscreenTaskIDs {
            favoriteWeatherTasks[id]?.cancel()
            favoriteWeatherTasks[id] = nil
        }

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
                guard let annotation = mapView.annotations.compactMap({ $0 as? FavoritePinAnnotation }).first(where: { $0.id == id }) else {
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
        if let view = mapView.view(for: annotation) as? BubbleAnnotationView {
            view.configureFavorite(with: annotation)
        }
    }

    private func areFavoritePinsVisible(on mapView: MKMapView) -> Bool {
        mapView.region.span.latitudeDelta <= favoritePinsMaxLatitudeDelta
    }

    private func applyFavoritePinVisibility(on mapView: MKMapView) {
        let visible = areFavoritePinsVisible(on: mapView)
        for annotation in mapView.annotations.compactMap({ $0 as? FavoritePinAnnotation }) {
            annotation.isVisibleAtCurrentZoom = visible
            if let view = mapView.view(for: annotation) as? BubbleAnnotationView {
                view.configureFavorite(with: annotation)
            }
        }
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

private struct WeatherMapUIKitBridge: UIViewRepresentable {
    @ObservedObject var viewModel: WeatherMapViewModel

    func makeCoordinator() -> Coordinator {
        Coordinator(viewModel: viewModel)
    }

    func makeUIView(context: Context) -> MKMapView {
        let mapView = MKMapView(frame: .zero)
        mapView.delegate = context.coordinator
        mapView.showsUserLocation = true
        mapView.pointOfInterestFilter = .excludingAll
        mapView.isRotateEnabled = false

        viewModel.bind(mapView: mapView)

        let longPress = UILongPressGestureRecognizer(
            target: context.coordinator,
            action: #selector(Coordinator.handleLongPress(_:))
        )
        longPress.minimumPressDuration = 0.4
        longPress.delegate = context.coordinator
        mapView.addGestureRecognizer(longPress)

        return mapView
    }

    func updateUIView(_ : MKMapView, context: Context) {
        // Intentionally empty: synchronization runs through explicit SwiftUI intents.
    }

    @MainActor
    final class Coordinator: NSObject, MKMapViewDelegate, UIGestureRecognizerDelegate {
        private let viewModel: WeatherMapViewModel

        init(viewModel: WeatherMapViewModel) {
            self.viewModel = viewModel
        }

        func mapView(_ mapView: MKMapView, regionDidChangeAnimated animated: Bool) {
            viewModel.handleRegionDidChange(on: mapView)
        }

        func mapView(_ mapView: MKMapView, rendererFor overlay: MKOverlay) -> MKOverlayRenderer {
            if let temperatureOverlay = overlay as? TemperatureImageOverlay {
                return TemperatureOverlayRenderer(overlay: temperatureOverlay)
            }
            return MKOverlayRenderer(overlay: overlay)
        }

        func mapView(_ mapView: MKMapView, didAdd views: [MKAnnotationView]) {
            for view in views where view.annotation is PreviewPinAnnotation {
                view.alpha = 0
                view.transform = CGAffineTransform(scaleX: 0.85, y: 0.85)
                UIView.animate(
                    withDuration: 0.35,
                    delay: 0,
                    usingSpringWithDamping: 0.7,
                    initialSpringVelocity: 0,
                    options: [.allowUserInteraction],
                    animations: {
                        view.alpha = 1
                        view.transform = .identity
                    },
                    completion: nil
                )
            }
        }

        func mapView(_ mapView: MKMapView, viewFor annotation: MKAnnotation) -> MKAnnotationView? {
            if let preview = annotation as? PreviewPinAnnotation {
                let reuseID = BubbleAnnotationView.previewReuseID
                let view: BubbleAnnotationView
                if let reused = mapView.dequeueReusableAnnotationView(withIdentifier: reuseID) as? BubbleAnnotationView {
                    view = reused
                } else {
                    view = BubbleAnnotationView(annotation: annotation, reuseIdentifier: reuseID)
                }
                view.annotation = preview
                view.configurePreview(with: preview, onTap: { [weak self, weak mapView] in
                    guard let self, let mapView else { return }
                    self.viewModel.dismissPreview(on: mapView)
                })
                return view
            }

            if let favorite = annotation as? FavoritePinAnnotation {
                let reuseID = BubbleAnnotationView.favoriteReuseID
                let view: BubbleAnnotationView
                if let reused = mapView.dequeueReusableAnnotationView(withIdentifier: reuseID) as? BubbleAnnotationView {
                    view = reused
                } else {
                    view = BubbleAnnotationView(annotation: annotation, reuseIdentifier: reuseID)
                }
                view.annotation = favorite
                view.configureFavorite(with: favorite)
                return view
            }
            return nil
        }

        @objc func handleLongPress(_ recognizer: UILongPressGestureRecognizer) {
            viewModel.handleLongPress(recognizer)
        }

        func gestureRecognizer(
            _ gestureRecognizer: UIGestureRecognizer,
            shouldReceive touch: UITouch
        ) -> Bool {
            guard gestureRecognizer is UILongPressGestureRecognizer else { return true }
            var view = touch.view
            while let candidate = view {
                if let annotationView = candidate as? MKAnnotationView,
                   annotationView.annotation is PreviewPinAnnotation || annotationView.annotation is FavoritePinAnnotation
                {
                    return false
                }
                view = candidate.superview
            }
            return true
        }

        func gestureRecognizer(
            _ gestureRecognizer: UIGestureRecognizer,
            shouldRecognizeSimultaneouslyWith other: UIGestureRecognizer
        ) -> Bool {
            true
        }
    }
}

private nonisolated final class FavoritePinAnnotation: NSObject, MKAnnotation {
    let id: UUID
    let title: String?
    let coordinate: CLLocationCoordinate2D
    var weather: FavoritePinWeather?
    var isVisibleAtCurrentZoom: Bool = true

    init(id: UUID, title: String, coordinate: CLLocationCoordinate2D, weather: FavoritePinWeather?) {
        self.id = id
        self.title = title
        self.coordinate = coordinate
        self.weather = weather
        super.init()
    }
}

private struct FavoriteWeatherPinBubbleView: View {
    let currentText: String
    let rangeText: String

    var body: some View {
        VStack(spacing: 2) {
            VStack(spacing: 0) {
                Text(currentText)
                    .font(.system(size: 19, weight: .semibold))
                    .frame(maxWidth: .infinity)
                    .padding(.top, 3)

                Text(rangeText)
                    .font(.system(size: 10, weight: .medium, design: .monospaced))
                    .foregroundStyle(Color.white.opacity(0.88))
                    .frame(maxWidth: .infinity)
            }
            .frame(width: 86, height: 44)
            .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
            .overlay {
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .stroke(Color.white.opacity(0.26), lineWidth: 0.5)
            }

            Circle()
                .fill(Color(red: 0.21, green: 0.69, blue: 1.0))
                .overlay {
                    Circle().stroke(Color.white.opacity(0.9), lineWidth: 1.0)
                }
                .frame(width: 8, height: 8)
        }
        .foregroundStyle(.white)
        .frame(width: 86, height: 54)
    }
}

private enum PreviewLoadState {
    case loading
    case loaded
    case failed
}

private struct PreviewWeather: Equatable {
    let current: Int?
    let conditionText: String?
    let symbolCode: String?
    let low: Int?
    let high: Int?

    static func from(response: WeatherResponse) -> PreviewWeather {
        let today = response.dailyForecast.first
        return PreviewWeather(
            current: roundTemperature(response.current.resolvedTemperature ?? response.hourlyForecast.first?.temperature),
            conditionText: WeatherSymbols.conditionDescription(from: response),
            symbolCode: WeatherSymbols.primarySymbol(from: response),
            low: roundTemperature(today?.low),
            high: roundTemperature(today?.high)
        )
    }

    private static func roundTemperature(_ value: Double?) -> Int? {
        guard let value else { return nil }
        return Int(value.rounded())
    }
}

private func formatFallbackPlaceName(_ coordinate: CLLocationCoordinate2D) -> String {
    String(
        format: "%.2f, %.2f",
        locale: Locale(identifier: "en_US_POSIX"),
        coordinate.latitude,
        coordinate.longitude
    )
}

private final class PreviewPinAnnotation: NSObject, MKAnnotation {
    @objc dynamic var coordinate: CLLocationCoordinate2D
    var placeName: String?
    var weather: PreviewWeather?
    var loadState: PreviewLoadState

    init(coordinate: CLLocationCoordinate2D) {
        self.coordinate = coordinate
        self.placeName = nil
        self.weather = nil
        self.loadState = .loading
        super.init()
    }
}

private struct PreviewPinBubbleView: View {
    let placeName: String
    let conditionText: String
    let symbolSystemName: String?
    let tempText: String
    let rangeText: String
    let onTap: (() -> Void)?

    var body: some View {
        VStack(spacing: 2) {
            VStack(spacing: 0) {
                Text(placeName)
                    .font(.system(size: 13, weight: .semibold))
                    .lineLimit(1)
                    .frame(maxWidth: .infinity)
                    .padding(.top, 6)

                HStack(spacing: 4) {
                    if let symbolSystemName {
                        Image(systemName: symbolSystemName)
                            .font(.system(size: 12, weight: .regular))
                    }
                    Text(conditionText)
                        .font(.system(size: 12, weight: .regular))
                        .lineLimit(1)
                }
                .foregroundStyle(Color.white.opacity(0.88))
                .frame(maxWidth: .infinity)
                .padding(.top, 2)

                Text(tempText)
                    .font(.system(size: 28, weight: .semibold))
                    .frame(maxWidth: .infinity)
                    .padding(.top, 2)

                Text(rangeText)
                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                    .foregroundStyle(Color.white.opacity(0.88))
                    .frame(maxWidth: .infinity)
            }
            .frame(width: 160, height: 92)
            .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
            .overlay {
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .stroke(Color.white.opacity(0.26), lineWidth: 0.5)
            }
            .onTapGesture {
                onTap?()
            }

            Circle()
                .fill(Color(red: 0.21, green: 0.69, blue: 1.0))
                .overlay {
                    Circle().stroke(Color.white.opacity(0.9), lineWidth: 1.0)
                }
                .frame(width: 8, height: 8)
        }
        .foregroundStyle(.white)
        .frame(width: 160, height: 104)
    }
}

private final class BubbleAnnotationView: MKAnnotationView {
    static let favoriteReuseID = "FavoriteWeatherPin"
    static let previewReuseID = "PreviewPin"

    private var onPreviewTap: (() -> Void)?
    private var hostingController: UIHostingController<AnyView>?

    override init(annotation: (any MKAnnotation)?, reuseIdentifier: String?) {
        super.init(annotation: annotation, reuseIdentifier: reuseIdentifier)
        setupView()
    }

    required init?(coder: NSCoder) {
        super.init(coder: coder)
        setupView()
    }

    override func prepareForReuse() {
        super.prepareForReuse()
        onPreviewTap = nil
        isHidden = false
        alpha = 1
        setHostedContent(AnyView(EmptyView()), isUserInteractionEnabled: false)
    }

    func configureFavorite(with annotation: FavoritePinAnnotation) {
        let currentText = annotation.weather?.currentText ?? "--°"
        let rangeText = annotation.weather?.rangeText ?? "L --°  H --°"
        centerOffset = CGPoint(x: 0, y: -26)
        bounds = CGRect(x: 0, y: 0, width: 86, height: 54)
        setHostedContent(
            AnyView(
                FavoriteWeatherPinBubbleView(
                    currentText: currentText,
                    rangeText: rangeText
                )
            ),
            isUserInteractionEnabled: false
        )

        isHidden = !annotation.isVisibleAtCurrentZoom
        alpha = annotation.isVisibleAtCurrentZoom ? 1 : 0
        accessibilityLabel = annotation.title ?? "Favorite location"
        accessibilityValue = "\(currentText), \(rangeText)"
    }

    func configurePreview(with annotation: PreviewPinAnnotation, onTap: (() -> Void)? = nil) {
        if let onTap {
            onPreviewTap = onTap
        }

        let placeName = annotation.placeName ?? "—"
        var conditionText = "Loading…"
        var symbolSystemName: String?
        var tempText = "--°"
        var rangeText = "L --°  H --°"

        switch annotation.loadState {
        case .loading:
            break
        case .failed:
            conditionText = "Weather unavailable"
        case .loaded:
            let weather = annotation.weather
            conditionText = weather?.conditionText ?? "—"
            if let code = weather?.symbolCode {
                symbolSystemName = SmartSymbol.systemImageName(for: code)
            }
            tempText = formatTemp(weather?.current)
            rangeText = "L \(formatTemp(weather?.low))  H \(formatTemp(weather?.high))"
        }

        centerOffset = CGPoint(x: 0, y: -54)
        bounds = CGRect(x: 0, y: 0, width: 160, height: 104)
        setHostedContent(
            AnyView(
                PreviewPinBubbleView(
                    placeName: placeName,
                    conditionText: conditionText,
                    symbolSystemName: symbolSystemName,
                    tempText: tempText,
                    rangeText: rangeText,
                    onTap: onPreviewTap
                )
            ),
            isUserInteractionEnabled: true
        )

        accessibilityLabel = placeName
        accessibilityValue = "\(tempText), \(conditionText)"
    }

    private func formatTemp(_ value: Int?) -> String {
        guard let value else { return "--°" }
        return "\(value)°"
    }

    private func setupView() {
        backgroundColor = .clear
        canShowCallout = false
        collisionMode = .none
        displayPriority = .required
    }

    private func setHostedContent(_ rootView: AnyView, isUserInteractionEnabled: Bool) {
        if let hostingController {
            hostingController.rootView = rootView
            hostingController.view.isUserInteractionEnabled = isUserInteractionEnabled
            hostingController.view.frame = bounds
            return
        }

        let controller = UIHostingController(rootView: rootView)
        controller.view.backgroundColor = .clear
        controller.view.isUserInteractionEnabled = isUserInteractionEnabled
        controller.view.frame = bounds
        controller.view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        addSubview(controller.view)
        hostingController = controller
    }
}
