import MapKit
import UIKit

/// Legacy wrapper retained for compatibility while Metal rendering is routed
/// through MapKit overlays.
final class TemperatureMetalOverlayView: UIView {
    let renderer: TemperatureMetalRenderer?
    var isRendererAvailable: Bool { renderer != nil }

    override init(frame: CGRect) {
        renderer = TemperatureMetalRenderer()
        super.init(frame: frame)
        isUserInteractionEnabled = false
        backgroundColor = .clear
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    func setSamples(_ samples: [TemperatureSample]) {
        renderer?.setSamples(samples)
    }

    func updateBounds(for mapView: MKMapView) { }
}
