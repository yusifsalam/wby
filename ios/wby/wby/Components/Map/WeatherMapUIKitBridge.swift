import MapKit
import SwiftUI
import UIKit

struct WeatherMapUIKitBridge: UIViewRepresentable {
    @ObservedObject var viewModel: WeatherMapViewModel

    func makeCoordinator() -> Coordinator { Coordinator(viewModel: viewModel) }

    func makeUIView(context: Context) -> MKMapView {
        let mapView = MKMapView(frame: .zero)
        mapView.delegate = context.coordinator
        mapView.showsUserLocation = true
        mapView.pointOfInterestFilter = .includingAll
        mapView.isRotateEnabled = false
        mapView.isPitchEnabled = false

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

    func updateUIView(_ : MKMapView, context: Context) { }

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
                let view = dequeueBubbleView(
                    in: mapView,
                    reuseID: BubbleAnnotationView.previewReuseID,
                    annotation: annotation
                )
                view.annotation = preview
                view.configurePreview(with: preview, onTap: { [weak self, weak mapView] in
                    guard let self, let mapView else { return }
                    self.viewModel.dismissPreview(on: mapView)
                })
                return view
            }

            if let favorite = annotation as? FavoritePinAnnotation {
                let view = dequeueBubbleView(
                    in: mapView,
                    reuseID: BubbleAnnotationView.favoriteReuseID,
                    annotation: annotation
                )
                view.annotation = favorite
                view.configureFavorite(with: favorite)
                return view
            }
            return nil
        }

        private func dequeueBubbleView(
            in mapView: MKMapView,
            reuseID: String,
            annotation: MKAnnotation
        ) -> BubbleAnnotationView {
            if let view = mapView.dequeueReusableAnnotationView(withIdentifier: reuseID) as? BubbleAnnotationView {
                return view
            }
            return BubbleAnnotationView(annotation: annotation, reuseIdentifier: reuseID)
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
        ) -> Bool { true }
    }
}
