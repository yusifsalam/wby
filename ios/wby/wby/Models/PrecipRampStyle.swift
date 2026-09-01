import Foundation

/// Color style for the precipitation grid overlays: the app's smooth gradient,
/// or the stepped radar classes matching FMI's WMS tiles. Applies to both the
/// 1h radar layer and the 12h forecast layer.
enum PrecipRampStyle: String, CaseIterable {
    case smooth
    case stepped

    static let storageKey = "weatherMap.precipRampStyle"

    var displayName: String {
        switch self {
        case .smooth: return "Smooth"
        case .stepped: return "Stepped"
        }
    }

    static func load(defaults: UserDefaults = .standard) -> PrecipRampStyle {
        if let raw = defaults.string(forKey: storageKey),
           let style = PrecipRampStyle(rawValue: raw) {
            return style
        }
        return .smooth
    }

    func save(defaults: UserDefaults = .standard) {
        defaults.set(rawValue, forKey: Self.storageKey)
    }
}
