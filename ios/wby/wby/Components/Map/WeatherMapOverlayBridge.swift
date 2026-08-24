import MapKit
import UIKit

// UIKit/MapKit bridge retained for custom raster temperature overlay rendering.
// The rest of WeatherMapView can be migrated independently.
nonisolated final class TemperatureImageOverlay: NSObject, MKOverlay {
    let coordinate: CLLocationCoordinate2D
    let boundingMapRect: MKMapRect
    let bbox: MapBBox
    // Temperature is a land field, so its raster is clipped to the Finland
    // coastline (matching the web map). Precipitation stays unclipped: rain
    // over the sea is real data.
    let clipsToFinland: Bool
    var image: UIImage

    init(bbox: MapBBox, image: UIImage, clipsToFinland: Bool = false) {
        self.bbox = bbox
        self.image = image
        self.clipsToFinland = clipsToFinland
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

final class TemperatureOverlayRenderer: MKOverlayRenderer {
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
        if overlay.clipsToFinland, !FinlandOutline.rings.isEmpty {
            let path = CGMutablePath()
            for ring in FinlandOutline.rings {
                // Skip islands outside this tile or smaller than ~half a
                // drawing point at the current zoom.
                guard ring.boundingRect.intersects(mapRect) else { continue }
                let size = max(ring.boundingRect.width, ring.boundingRect.height)
                guard size * Double(zoomScale) >= 0.5 else { continue }
                path.move(to: point(for: ring.points[0]))
                for mapPoint in ring.points.dropFirst() {
                    path.addLine(to: point(for: mapPoint))
                }
                path.closeSubpath()
            }
            // No land in this tile. Bail out instead of clipping: CGContext's
            // clip() is a no-op on an empty path, which would draw unclipped.
            guard !path.isEmpty else { return }
            context.addPath(path)
            context.clip()
        }
        // Server raster is generated north-at-top; CGContext image drawing here needs an explicit Y flip.
        context.translateBy(x: drawRect.minX, y: drawRect.maxY)
        context.scaleBy(x: 1, y: -1)
        context.draw(cgImage, in: CGRect(origin: .zero, size: drawRect.size))
    }
}
