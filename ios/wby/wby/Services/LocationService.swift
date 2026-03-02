import CoreLocation
import MapKit

@Observable
final class LocationService: NSObject, CLLocationManagerDelegate {
    private let manager = CLLocationManager()

    var coordinate: CLLocationCoordinate2D?
    var altitudeMeters: Double?
    var placeName: String?
    var error: Error?

    private var pendingContinuations: [CheckedContinuation<CLLocationCoordinate2D?, Never>] = []

    override init() {
        super.init()
        manager.delegate = self
        manager.desiredAccuracy = kCLLocationAccuracyKilometer
    }

    /// One-shot location request. If authorized, suspends until CLLocationManager delivers a fix.
    /// If not yet authorized, triggers the permission prompt and returns the last known coordinate.
    func requestFreshLocation() async -> CLLocationCoordinate2D? {
        switch manager.authorizationStatus {
        case .notDetermined:
            manager.requestWhenInUseAuthorization()
            return coordinate
        case .authorizedWhenInUse, .authorizedAlways:
            return await withCheckedContinuation { continuation in
                pendingContinuations.append(continuation)
                manager.requestLocation()
            }
        default:
            return coordinate
        }
    }

    func locationManagerDidChangeAuthorization(_ manager: CLLocationManager) {
        switch manager.authorizationStatus {
        case .authorizedWhenInUse, .authorizedAlways:
            manager.requestLocation()
        default:
            break
        }
    }

    func locationManager(_ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        guard let location = locations.last else { return }
        coordinate = location.coordinate
        altitudeMeters = Self.validAltitude(from: location)
        reverseGeocode(location)
        let waiting = pendingContinuations
        pendingContinuations.removeAll()
        for c in waiting { c.resume(returning: location.coordinate) }
    }

    func locationManager(_ manager: CLLocationManager, didFailWithError error: Error) {
        self.error = error
        let waiting = pendingContinuations
        pendingContinuations.removeAll()
        for c in waiting { c.resume(returning: nil) }
    }

    private func reverseGeocode(_ location: CLLocation) {
        guard let request = MKReverseGeocodingRequest(location: location) else { return }
        request.getMapItems { [weak self] items, _ in
            guard let mapItem = items?.first else { return }
            self?.placeName = Self.displayAreaName(from: mapItem)
        }
    }

    private static func displayAreaName(from mapItem: MKMapItem) -> String? {
        if let city = nonEmpty(mapItem.addressRepresentations?.cityName) {
            return city
        }
        if let cityWithContext = nonEmpty(mapItem.addressRepresentations?.cityWithContext) {
            return cityWithContext
        }
        if let shortAddress = nonEmpty(mapItem.address?.shortAddress) {
            return shortAddress
        }
        if let region = nonEmpty(mapItem.addressRepresentations?.regionName) {
            return region
        }
        return nonEmpty(mapItem.address?.fullAddress)
    }

    private static func nonEmpty(_ value: String?) -> String? {
        guard let trimmed = value?.trimmingCharacters(in: .whitespacesAndNewlines), !trimmed.isEmpty else {
            return nil
        }
        return trimmed
    }

    private static func validAltitude(from location: CLLocation) -> Double? {
        guard location.verticalAccuracy >= 0 else { return nil }
        let altitude = location.altitude
        guard altitude.isFinite else { return nil }
        return altitude
    }
}
