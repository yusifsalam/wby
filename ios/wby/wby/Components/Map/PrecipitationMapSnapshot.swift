import CoreLocation
import MapKit
import UIKit

struct PrecipitationMapSnapshot {
    let image: UIImage
    let dataTime: Date?
    let maxRate: Double?
}

/// Composites the latest radar rain-rate raster over an `MKMapSnapshotter`
/// basemap centred on a location, for the precipitation card on the weather
/// page. Results are cached briefly so scrolling back to a page doesn't
/// re-render; a new radar frame lands every 5 minutes anyway.
@MainActor
final class PrecipitationMapSnapshotLoader {
    static let shared = PrecipitationMapSnapshotLoader()
    static let size = CGSize(width: 360, height: 210)

    private struct CacheEntry {
        let snapshot: PrecipitationMapSnapshot
        let createdAt: Date
    }

    private let renderer = TemperatureMetalRenderer()
    private var cache: [String: CacheEntry] = [:]
    private let maxAge: TimeInterval = 4 * 60
    private let regionMeters: CLLocationDistance = 280_000

    func snapshot(
        for coordinate: CLLocationCoordinate2D,
        style: UIUserInterfaceStyle,
        weatherService: WeatherService,
        force: Bool = false
    ) async -> PrecipitationMapSnapshot? {
        let ramp = PrecipRampStyle.load()
        let key = String(
            format: "%.2f,%.2f,%d,%@",
            coordinate.latitude, coordinate.longitude, style.rawValue, ramp.rawValue
        )
        if !force, let entry = cache[key], Date().timeIntervalSince(entry.createdAt) < maxAge {
            return entry.snapshot
        }

        async let radar = loadRadar(around: coordinate, weatherService: weatherService)
        guard let base = await baseSnapshot(for: coordinate, style: style) else { return nil }
        let response = await radar

        let overlay = response.flatMap { render($0, ramp: ramp) }
        let image = compose(base: base, coordinate: coordinate, overlay: overlay)
        let snapshot = PrecipitationMapSnapshot(
            image: image,
            dataTime: response?.dataTime,
            maxRate: overlay == nil ? nil : response?.max
        )
        cache[key] = CacheEntry(snapshot: snapshot, createdAt: Date())
        return snapshot
    }

    private struct RenderedOverlay {
        let image: UIImage
        let minLat: Double
        let maxLat: Double
        let minLon: Double
        let maxLon: Double
    }

    private func baseSnapshot(
        for coordinate: CLLocationCoordinate2D,
        style: UIUserInterfaceStyle
    ) async -> MKMapSnapshotter.Snapshot? {
        let options = MKMapSnapshotter.Options()
        options.region = MKCoordinateRegion(
            center: coordinate,
            latitudinalMeters: regionMeters,
            longitudinalMeters: regionMeters
        )
        options.size = Self.size
        options.mapType = .mutedStandard
        options.pointOfInterestFilter = .excludingAll
        options.showsBuildings = false
        options.traitCollection = UITraitCollection(userInterfaceStyle: style)
        return try? await MKMapSnapshotter(options: options).start()
    }

    private func loadRadar(
        around coordinate: CLLocationCoordinate2D,
        weatherService: WeatherService
    ) async -> PrecipitationForecastResponse? {
        let latPad = 1.7
        let lonPad = 7.0
        let bbox = MapBBox(
            minLon: coordinate.longitude - lonPad,
            minLat: coordinate.latitude - latPad,
            maxLon: coordinate.longitude + lonPad,
            maxLat: coordinate.latitude + latPad
        ).clampedToWorld()
        return try? await weatherService.fetchPrecipitationObservedGrid(
            bbox: bbox, width: 320, height: 192, time: nil
        )
    }

    private func render(_ response: PrecipitationForecastResponse, ramp: PrecipRampStyle) -> RenderedOverlay? {
        guard let renderer, let grid = response.grid,
              renderer.setGrid(grid, field: ramp == .stepped ? .precipitationStepped : .precipitation)
        else { return nil }
        let bounds = MercatorBounds(
            topMercY: MercatorBounds.mercatorY(lat: grid.maxLat),
            botMercY: MercatorBounds.mercatorY(lat: grid.minLat),
            leftLon: grid.minLon,
            rightLon: grid.maxLon
        )
        guard let image = renderer.renderImage(bounds: bounds, width: 640, height: 384) else { return nil }
        return RenderedOverlay(
            image: image,
            minLat: grid.minLat, maxLat: grid.maxLat,
            minLon: grid.minLon, maxLon: grid.maxLon
        )
    }

    private func compose(
        base: MKMapSnapshotter.Snapshot,
        coordinate: CLLocationCoordinate2D,
        overlay: RenderedOverlay?
    ) -> UIImage {
        let size = base.image.size
        let format = UIGraphicsImageRendererFormat()
        format.scale = base.image.scale
        return UIGraphicsImageRenderer(size: size, format: format).image { ctx in
            base.image.draw(at: .zero)

            if let overlay {
                let topLeft = base.point(for: CLLocationCoordinate2D(latitude: overlay.maxLat, longitude: overlay.minLon))
                let bottomRight = base.point(for: CLLocationCoordinate2D(latitude: overlay.minLat, longitude: overlay.maxLon))
                let rect = CGRect(
                    x: topLeft.x,
                    y: topLeft.y,
                    width: bottomRight.x - topLeft.x,
                    height: bottomRight.y - topLeft.y
                )
                ctx.cgContext.interpolationQuality = .high
                overlay.image.draw(in: rect)
            }

            let pin = base.point(for: coordinate)
            let dotSize: CGFloat = 14
            let dotRect = CGRect(x: pin.x - dotSize / 2, y: pin.y - dotSize / 2, width: dotSize, height: dotSize)
            ctx.cgContext.setFillColor(UIColor.systemBlue.cgColor)
            ctx.cgContext.fillEllipse(in: dotRect)
            ctx.cgContext.setStrokeColor(UIColor.white.cgColor)
            ctx.cgContext.setLineWidth(3)
            ctx.cgContext.strokeEllipse(in: dotRect)
        }
    }
}
