import CoreLocation

enum WeatherLocation {
    case gps
    case favorite(FavoriteLocation)
}
