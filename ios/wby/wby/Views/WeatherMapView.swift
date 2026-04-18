import CoreLocation
import MapKit
import SwiftUI
import UIKit

struct WeatherMapView: View {
    let locationService: LocationService
    let favoritesStore: FavoritesStore
    let weatherService: WeatherService
    private let overlayTimeZone = TimeZone(identifier: "Europe/Helsinki")!

    @Environment(\.dismiss) private var dismiss
    @State private var meta: OverlayMeta?

    var body: some View {
        ZStack {
            WeatherMapContainer(
                currentCoordinate: locationService.coordinate,
                favorites: favoritesStore.favorites,
                weatherService: weatherService,
                onMetaUpdate: { meta = $0 }
            )
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
                    if let meta {
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
            _ = await locationService.requestFreshLocation()
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

private struct WeatherMapContainer: UIViewRepresentable {
    let currentCoordinate: CLLocationCoordinate2D?
    let favorites: [FavoriteLocation]
    let weatherService: WeatherService
    let onMetaUpdate: (OverlayMeta?) -> Void

    func makeCoordinator() -> Coordinator {
        Coordinator(
            overlayService: MapOverlayService(weatherService: weatherService),
            weatherService: weatherService,
            onMetaUpdate: onMetaUpdate
        )
    }

    func makeUIView(context: Context) -> MKMapView {
        let mapView = MKMapView(frame: .zero)
        mapView.delegate = context.coordinator
        mapView.showsUserLocation = true
        mapView.pointOfInterestFilter = .excludingAll
        mapView.isRotateEnabled = false

        if let coordinate = currentCoordinate {
            mapView.setRegion(
                MKCoordinateRegion(
                    center: coordinate,
                    span: MKCoordinateSpan(latitudeDelta: 1.2, longitudeDelta: 1.2)
                ),
                animated: false
            )
        } else {
            mapView.setRegion(
                MKCoordinateRegion(
                    center: CLLocationCoordinate2D(latitude: 64.9, longitude: 25.5),
                    span: MKCoordinateSpan(latitudeDelta: 9.0, longitudeDelta: 11.0)
                ),
                animated: false
            )
        }

        context.coordinator.bind(mapView: mapView)
        context.coordinator.updateFavoriteAnnotations(on: mapView, favorites: favorites)
        context.coordinator.scheduleOverlayRefresh(on: mapView)
        return mapView
    }

    func updateUIView(_ mapView: MKMapView, context: Context) {
        context.coordinator.bind(mapView: mapView)
        context.coordinator.updateFavoriteAnnotations(on: mapView, favorites: favorites)
        context.coordinator.scheduleOverlayRefresh(on: mapView)
    }

    @MainActor
    final class Coordinator: NSObject, MKMapViewDelegate {
        private let favoriteWeatherTTL: TimeInterval = 10 * 60
        private let favoritePinsMaxLatitudeDelta: CLLocationDegrees = 5.8
        private let overlayRefreshInterval: TimeInterval = 3 * 60
        private let overlaySize: Int = 512
        private let overlayService: MapOverlayService
        private let weatherService: WeatherService
        private let onMetaUpdate: (OverlayMeta?) -> Void
        private weak var mapView: MKMapView?
        private var overlayTask: Task<Void, Never>?
        private var overlayLastFetchedAt: Date?
        private var favoriteWeatherCache: [UUID: CachedFavoritePinWeather] = [:]
        private var favoriteWeatherTasks: [UUID: Task<Void, Never>] = [:]

        init(
            overlayService: MapOverlayService,
            weatherService: WeatherService,
            onMetaUpdate: @escaping (OverlayMeta?) -> Void
        ) {
            self.overlayService = overlayService
            self.weatherService = weatherService
            self.onMetaUpdate = onMetaUpdate
        }

        deinit {
            overlayTask?.cancel()
            for task in favoriteWeatherTasks.values {
                task.cancel()
            }
        }

        func bind(mapView: MKMapView) {
            self.mapView = mapView
        }

        func mapView(_ mapView: MKMapView, regionDidChangeAnimated animated: Bool) {
            applyFavoritePinVisibility(on: mapView)
            refreshVisibleFavoriteWeather(on: mapView)
            scheduleOverlayRefresh(on: mapView)
        }

        func mapView(_ mapView: MKMapView, rendererFor overlay: MKOverlay) -> MKOverlayRenderer {
            if let temperatureOverlay = overlay as? TemperatureImageOverlay {
                return TemperatureOverlayRenderer(overlay: temperatureOverlay)
            }
            return MKOverlayRenderer(overlay: overlay)
        }

        func mapView(_ mapView: MKMapView, viewFor annotation: MKAnnotation) -> MKAnnotationView? {
            guard let favorite = annotation as? FavoritePinAnnotation else { return nil }
            let reuseID = FavoriteWeatherAnnotationView.reuseID
            let view: FavoriteWeatherAnnotationView
            if let reused = mapView.dequeueReusableAnnotationView(withIdentifier: reuseID) as? FavoriteWeatherAnnotationView {
                view = reused
            } else {
                view = FavoriteWeatherAnnotationView(annotation: annotation, reuseIdentifier: reuseID)
            }
            view.annotation = favorite
            view.configure(with: favorite)
            return view
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
                    await MainActor.run {
                        self.overlayLastFetchedAt = Date()
                        self.applyOverlay(
                            image: image,
                            bbox: overlayImage.bbox,
                            on: mapView
                        )
                        self.onMetaUpdate(
                            OverlayMeta(
                                dataTime: overlayImage.dataTime,
                                minTemp: overlayImage.minTemp,
                                maxTemp: overlayImage.maxTemp
                            )
                        )
                    }
                } catch {
                    // Keep previous successful overlay visible on fetch errors.
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
            if let view = mapView.view(for: annotation) as? FavoriteWeatherAnnotationView {
                view.configure(with: annotation)
            }
        }

        private func areFavoritePinsVisible(on mapView: MKMapView) -> Bool {
            mapView.region.span.latitudeDelta <= favoritePinsMaxLatitudeDelta
        }

        private func applyFavoritePinVisibility(on mapView: MKMapView) {
            let visible = areFavoritePinsVisible(on: mapView)
            for annotation in mapView.annotations.compactMap({ $0 as? FavoritePinAnnotation }) {
                annotation.isVisibleAtCurrentZoom = visible
                if let view = mapView.view(for: annotation) as? FavoriteWeatherAnnotationView {
                    view.configure(with: annotation)
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

private final class FavoriteWeatherAnnotationView: MKAnnotationView {
    static let reuseID = "FavoriteWeatherPin"

    private let bubbleView = UIVisualEffectView(effect: UIBlurEffect(style: .systemUltraThinMaterialDark))
    private let currentLabel = UILabel()
    private let rangeLabel = UILabel()
    private let anchorDot = UIView()

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
        currentLabel.text = "--°"
        rangeLabel.text = "L --°  H --°"
        isHidden = false
        alpha = 1
    }

    override func layoutSubviews() {
        super.layoutSubviews()

        let bubbleWidth: CGFloat = 86
        let bubbleHeight: CGFloat = 44
        let dotSize: CGFloat = 8

        bubbleView.frame = CGRect(x: 0, y: 0, width: bubbleWidth, height: bubbleHeight)
        currentLabel.frame = CGRect(x: 8, y: 3, width: bubbleWidth - 16, height: 22)
        rangeLabel.frame = CGRect(x: 8, y: 23, width: bubbleWidth - 16, height: 16)
        anchorDot.frame = CGRect(x: (bubbleWidth - dotSize) / 2, y: bubbleHeight + 2, width: dotSize, height: dotSize)
        anchorDot.layer.cornerRadius = dotSize / 2
    }

    func configure(with annotation: FavoritePinAnnotation) {
        currentLabel.text = annotation.weather?.currentText ?? "--°"
        rangeLabel.text = annotation.weather?.rangeText ?? "L --°  H --°"
        isHidden = !annotation.isVisibleAtCurrentZoom
        alpha = annotation.isVisibleAtCurrentZoom ? 1 : 0
        accessibilityLabel = annotation.title ?? "Favorite location"
        accessibilityValue = "\(currentLabel.text ?? "--°"), \(rangeLabel.text ?? "L --° H --°")"
    }

    private func setupView() {
        backgroundColor = .clear
        canShowCallout = false
        collisionMode = .none
        displayPriority = .required
        centerOffset = CGPoint(x: 0, y: -26)
        bounds = CGRect(x: 0, y: 0, width: 86, height: 54)

        bubbleView.clipsToBounds = true
        bubbleView.layer.cornerRadius = 14
        bubbleView.layer.cornerCurve = .continuous
        bubbleView.layer.borderColor = UIColor.white.withAlphaComponent(0.26).cgColor
        bubbleView.layer.borderWidth = 0.5
        addSubview(bubbleView)

        currentLabel.font = .systemFont(ofSize: 19, weight: .semibold)
        currentLabel.textColor = .white
        currentLabel.textAlignment = .center
        currentLabel.adjustsFontSizeToFitWidth = true
        currentLabel.minimumScaleFactor = 0.8
        bubbleView.contentView.addSubview(currentLabel)

        rangeLabel.font = .monospacedDigitSystemFont(ofSize: 10, weight: .medium)
        rangeLabel.textColor = UIColor.white.withAlphaComponent(0.88)
        rangeLabel.textAlignment = .center
        bubbleView.contentView.addSubview(rangeLabel)

        anchorDot.backgroundColor = UIColor(red: 0.21, green: 0.69, blue: 1.0, alpha: 1.0)
        anchorDot.layer.borderColor = UIColor.white.withAlphaComponent(0.9).cgColor
        anchorDot.layer.borderWidth = 1.0
        addSubview(anchorDot)
    }

}

private nonisolated final class TemperatureImageOverlay: NSObject, MKOverlay {
    let coordinate: CLLocationCoordinate2D
    let boundingMapRect: MKMapRect
    let image: UIImage

    init(bbox: MapBBox, image: UIImage) {
        self.image = image
        coordinate = CLLocationCoordinate2D(
            latitude: (bbox.minLat + bbox.maxLat) / 2.0,
            longitude: (bbox.minLon + bbox.maxLon) / 2.0
        )

        let topLeft = MKMapPoint(CLLocationCoordinate2D(latitude: bbox.maxLat, longitude: bbox.minLon))
        let bottomRight = MKMapPoint(CLLocationCoordinate2D(latitude: bbox.minLat, longitude: bbox.maxLon))

        boundingMapRect = MKMapRect(
            x: min(topLeft.x, bottomRight.x),
            y: min(topLeft.y, bottomRight.y),
            width: abs(bottomRight.x - topLeft.x),
            height: abs(bottomRight.y - topLeft.y)
        )
        super.init()
    }
}

private final class TemperatureOverlayRenderer: MKOverlayRenderer {
    nonisolated override init(overlay: any MKOverlay) {
        super.init(overlay: overlay)
    }

    nonisolated override func draw(_ mapRect: MKMapRect, zoomScale: MKZoomScale, in context: CGContext) {
        guard
            let overlay = overlay as? TemperatureImageOverlay,
            let cgImage = overlay.image.cgImage
        else { return }

        let overlayRect = overlay.boundingMapRect
        guard mapRect.intersects(overlayRect) else { return }

        let drawRect = rect(for: overlayRect)
        let tileRect = rect(for: mapRect)
        context.saveGState()
        defer { context.restoreGState() }
        context.setBlendMode(.normal)
        context.clip(to: tileRect)
        // Server raster is generated north-at-top; CGContext image drawing here needs an explicit Y flip.
        context.translateBy(x: drawRect.minX, y: drawRect.maxY)
        context.scaleBy(x: 1, y: -1)
        context.draw(cgImage, in: CGRect(origin: .zero, size: drawRect.size))
    }
}
