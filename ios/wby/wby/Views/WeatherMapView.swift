import CoreLocation
import MapKit
import SwiftUI
import UIKit

struct WeatherMapView: View {
    let locationService: LocationService
    let favoritesStore: FavoritesStore
    let weatherService: WeatherService

    @State private var meta: OverlayMeta?

    var body: some View {
        ZStack(alignment: .topTrailing) {
            WeatherMapContainer(
                currentCoordinate: locationService.coordinate,
                favorites: favoritesStore.favorites,
                weatherService: weatherService,
                onMetaUpdate: { meta = $0 }
            )
            .ignoresSafeArea()

            if let meta {
                VStack(alignment: .trailing, spacing: 4) {
                    if let min = meta.minTemp, let max = meta.maxTemp {
                        Text("\(Int(min.rounded()))° ... \(Int(max.rounded()))°")
                            .font(.caption2.bold())
                    }
                    if let dataTime = meta.dataTime {
                        Text(dataTime.formatted(date: .omitted, time: .shortened))
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                    }
                }
                .padding(10)
                .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 10))
                .padding()
            }
        }
        .navigationTitle("Weather Map")
        .navigationBarTitleDisplayMode(.inline)
        .task {
            _ = await locationService.requestFreshLocation()
        }
    }
}

private struct OverlayMeta: Equatable {
    let dataTime: Date?
    let minTemp: Double?
    let maxTemp: Double?
}

private struct WeatherMapContainer: UIViewRepresentable {
    let currentCoordinate: CLLocationCoordinate2D?
    let favorites: [FavoriteLocation]
    let weatherService: WeatherService
    let onMetaUpdate: (OverlayMeta?) -> Void

    func makeCoordinator() -> Coordinator {
        Coordinator(
            overlayService: MapOverlayService(weatherService: weatherService),
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

        context.coordinator.updateFavoriteAnnotations(on: mapView, favorites: favorites)
        context.coordinator.scheduleOverlayRefresh(on: mapView)
        return mapView
    }

    func updateUIView(_ mapView: MKMapView, context: Context) {
        context.coordinator.updateFavoriteAnnotations(on: mapView, favorites: favorites)
        context.coordinator.scheduleOverlayRefresh(on: mapView)
    }

    final class Coordinator: NSObject, MKMapViewDelegate {
        private let overlayService: MapOverlayService
        private let onMetaUpdate: (OverlayMeta?) -> Void
        private var refreshTask: Task<Void, Never>?

        init(overlayService: MapOverlayService, onMetaUpdate: @escaping (OverlayMeta?) -> Void) {
            self.overlayService = overlayService
            self.onMetaUpdate = onMetaUpdate
        }

        func mapView(_ mapView: MKMapView, regionDidChangeAnimated animated: Bool) {
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
            let reuseID = "FavoritePin"
            let view: MKMarkerAnnotationView
            if let reused = mapView.dequeueReusableAnnotationView(withIdentifier: reuseID) as? MKMarkerAnnotationView {
                view = reused
            } else {
                view = MKMarkerAnnotationView(annotation: annotation, reuseIdentifier: reuseID)
                view.canShowCallout = true
                view.markerTintColor = UIColor.systemBlue
                view.glyphImage = UIImage(systemName: "star.fill")
            }
            view.annotation = favorite
            return view
        }

        func updateFavoriteAnnotations(on mapView: MKMapView, favorites: [FavoriteLocation]) {
            let existing = mapView.annotations.compactMap { $0 as? FavoritePinAnnotation }
            if !existing.isEmpty {
                mapView.removeAnnotations(existing)
            }
            let newAnnotations = favorites.map {
                FavoritePinAnnotation(
                    id: $0.id,
                    title: $0.name,
                    coordinate: CLLocationCoordinate2D(latitude: $0.latitude, longitude: $0.longitude)
                )
            }
            mapView.addAnnotations(newAnnotations)
        }

        func scheduleOverlayRefresh(on mapView: MKMapView) {
            refreshTask?.cancel()

            let bbox = mapBBox(for: mapView)
            let scale = UIScreen.main.scale
            let width = max(120, Int(mapView.bounds.width * scale))
            let height = max(120, Int(mapView.bounds.height * scale))

            refreshTask = Task { [weak mapView] in
                try? await Task.sleep(nanoseconds: 400_000_000)
                guard !Task.isCancelled else { return }
                do {
                    let overlayImage = try await overlayService.fetchTemperatureOverlay(
                        bbox: bbox,
                        width: width,
                        height: height
                    )
                    guard !Task.isCancelled, let mapView, let image = UIImage(data: overlayImage.imageData) else { return }
                    await MainActor.run {
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

        private func applyOverlay(image: UIImage, bbox: MapBBox, on mapView: MKMapView) {
            let existing = mapView.overlays.compactMap { $0 as? TemperatureImageOverlay }
            if !existing.isEmpty {
                mapView.removeOverlays(existing)
            }
            mapView.addOverlay(TemperatureImageOverlay(bbox: bbox, image: image), level: .aboveRoads)
        }

        private func mapBBox(for mapView: MKMapView) -> MapBBox {
            let rect = mapView.visibleMapRect
            let topLeft = MKMapPoint(x: rect.minX, y: rect.minY).coordinate
            let bottomRight = MKMapPoint(x: rect.maxX, y: rect.maxY).coordinate
            return MapBBox(
                minLon: min(topLeft.longitude, bottomRight.longitude),
                minLat: min(topLeft.latitude, bottomRight.latitude),
                maxLon: max(topLeft.longitude, bottomRight.longitude),
                maxLat: max(topLeft.latitude, bottomRight.latitude)
            )
        }
    }
}

private final class FavoritePinAnnotation: NSObject, MKAnnotation {
    let id: UUID
    let title: String?
    let coordinate: CLLocationCoordinate2D

    init(id: UUID, title: String, coordinate: CLLocationCoordinate2D) {
        self.id = id
        self.title = title
        self.coordinate = coordinate
    }
}

private final class TemperatureImageOverlay: NSObject, MKOverlay {
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
    override func draw(_ mapRect: MKMapRect, zoomScale: MKZoomScale, in context: CGContext) {
        guard
            let overlay = overlay as? TemperatureImageOverlay,
            let cgImage = overlay.image.cgImage
        else { return }

        let drawRect = rect(for: overlay.boundingMapRect)
        context.setAlpha(0.74)
        context.draw(cgImage, in: drawRect)
    }
}
