import Foundation

enum OverlayMode: String, CaseIterable {
    case metal
    case png

    static let storageKey = "weatherMap.overlayMode"

    var displayName: String {
        switch self {
        case .metal: return "Metal"
        case .png: return "PNG"
        }
    }

    var usesMetal: Bool {
        get { self == .metal }
        set { self = newValue ? .metal : .png }
    }

    static func load(defaults: UserDefaults = .standard) -> OverlayMode {
        if let raw = defaults.string(forKey: storageKey),
           let mode = OverlayMode(rawValue: raw) {
            return mode
        }
        return .metal
    }

    func save(defaults: UserDefaults = .standard) {
        defaults.set(rawValue, forKey: Self.storageKey)
    }
}
