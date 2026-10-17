import Foundation

/// Color style for the Metal map overlays: the app's smooth gradients, or
/// stepped bands — 2 °C temperature bands, and for rain the radar classes
/// matching FMI's WMS tiles. Applies to every map layer.
enum MapRampStyle: String, CaseIterable {
    case smooth
    case stepped

    static let storageKey = "weatherMap.precipRampStyle"

    var displayName: String {
        switch self {
        case .smooth: return "Smooth"
        case .stepped: return "Stepped"
        }
    }

    static func load(defaults: UserDefaults = .standard) -> MapRampStyle {
        if let raw = defaults.string(forKey: storageKey),
           let style = MapRampStyle(rawValue: raw) {
            return style
        }
        return .smooth
    }

    func save(defaults: UserDefaults = .standard) {
        defaults.set(rawValue, forKey: Self.storageKey)
    }
}
