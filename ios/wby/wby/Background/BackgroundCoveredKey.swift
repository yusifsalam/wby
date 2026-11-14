import SwiftUI

/// Set by views that present a sheet over the weather pages, so the animated
/// background can pause while it is hidden.
nonisolated struct BackgroundCoveredKey: PreferenceKey {
    static let defaultValue = false

    static func reduce(value: inout Bool, nextValue: () -> Bool) {
        value = value || nextValue()
    }
}

extension View {
    func coversWeatherBackground(_ covered: Bool) -> some View {
        transformPreference(BackgroundCoveredKey.self) { $0 = $0 || covered }
    }
}
