import Foundation
import MapKit

nonisolated final class FavoritePinAnnotation: NSObject, MKAnnotation {
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

final class PreviewPinAnnotation: NSObject, MKAnnotation {
    @objc dynamic var coordinate: CLLocationCoordinate2D
    var placeName: String?
    var weather: PreviewWeather?
    var loadState: PreviewLoadState

    init(coordinate: CLLocationCoordinate2D) {
        self.coordinate = coordinate
        placeName = nil
        weather = nil
        loadState = .loading
        super.init()
    }
}

func formatFallbackPlaceName(_ coordinate: CLLocationCoordinate2D) -> String {
    String(format: "%.2f, %.2f", locale: Locale(identifier: "en_US_POSIX"), coordinate.latitude, coordinate.longitude)
}

extension MKMapView {
    var favoritePinAnnotations: [FavoritePinAnnotation] {
        annotations.compactMap { $0 as? FavoritePinAnnotation }
    }

    func favoritePinAnnotation(id: UUID) -> FavoritePinAnnotation? {
        favoritePinAnnotations.first { $0.id == id }
    }

    func bubbleView(for annotation: MKAnnotation) -> BubbleAnnotationView? {
        view(for: annotation) as? BubbleAnnotationView
    }
}
