import Foundation

enum MapLayerKind: String, CaseIterable {
    case temperature
    case precipitation

    static let storageKey = "weatherMap.layerKind"

    var displayName: String {
        switch self {
        case .temperature: return "Temperature"
        case .precipitation: return "Precipitation"
        }
    }

    var symbolName: String {
        switch self {
        case .temperature: return "thermometer.medium"
        case .precipitation: return "cloud.heavyrain.fill"
        }
    }

    var supportsMetalToggle: Bool {
        self == .temperature
    }

    /// How many timeline steps before "now" the scrubber covers.
    var scrubberPastSteps: Int {
        switch self {
        case .temperature: return 6
        case .precipitation: return 12
        }
    }

    /// How many timeline steps after "now" the scrubber covers.
    var scrubberFutureSteps: Int {
        switch self {
        case .temperature: return 12
        case .precipitation: return 12
        }
    }

    /// Length of one scrubber step in seconds.
    var scrubberStepSeconds: TimeInterval {
        switch self {
        case .temperature: return 3600
        case .precipitation: return 300
        }
    }

    static func load(defaults: UserDefaults = .standard) -> MapLayerKind {
        if let raw = defaults.string(forKey: storageKey),
           let kind = MapLayerKind(rawValue: raw) {
            return kind
        }
        return .temperature
    }

    func save(defaults: UserDefaults = .standard) {
        defaults.set(rawValue, forKey: Self.storageKey)
    }
}
