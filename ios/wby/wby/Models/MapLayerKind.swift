import Foundation

enum MapLayerKind: String, CaseIterable {
    case temperature
    case precipitation
    case precipitation12h

    static let storageKey = "weatherMap.layerKind"

    var displayName: String {
        switch self {
        case .temperature: return "Temperature"
        case .precipitation: return "Precipitation 1h"
        case .precipitation12h: return "Precipitation 12h"
        }
    }

    var symbolName: String {
        switch self {
        case .temperature: return "thermometer.medium"
        case .precipitation: return "cloud.heavyrain.fill"
        case .precipitation12h: return "cloud.sun.rain.fill"
        }
    }

    var supportsMetalToggle: Bool {
        self == .temperature
    }

    /// Whether the layer renders server PNG frames keyed by time (precipitation),
    /// as opposed to the temperature sample/overlay paths.
    var usesPrecipitationFrames: Bool {
        self == .precipitation || self == .precipitation12h
    }

    /// How many timeline steps before "now" the scrubber covers.
    var scrubberPastSteps: Int {
        switch self {
        case .temperature: return 6
        case .precipitation: return 12
        case .precipitation12h: return 0
        }
    }

    /// How many timeline steps after "now" the scrubber covers.
    var scrubberFutureSteps: Int {
        switch self {
        case .temperature: return 48
        case .precipitation: return 12
        case .precipitation12h: return 12
        }
    }

    /// Length of one scrubber step in seconds.
    var scrubberStepSeconds: TimeInterval {
        switch self {
        case .temperature: return 3600
        case .precipitation: return 300
        case .precipitation12h: return 3600
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
