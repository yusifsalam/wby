import CoreLocation

enum WeatherLocation: Sendable, Equatable, Hashable {
    case gps
    case favorite(FavoriteLocation)
}
