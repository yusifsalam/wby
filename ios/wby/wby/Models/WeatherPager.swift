import CoreGraphics
import Foundation
import Observation

enum WeatherPageID: Hashable {
    case gps
    case favorite(UUID)
}

/// State the weather pages share with the pager around them: each page's
/// scroll offset and background, and the request to open the map. Pages write
/// to it directly, so the pager passes them no closures and a pager update
/// leaves the pages untouched.
@Observable
final class WeatherPagerModel {
    var offsets: [WeatherPageID: CGFloat] = [:]
    var backgrounds: [WeatherPageID: WeatherBackgroundState] = [:]
    var showingMap = false
}
