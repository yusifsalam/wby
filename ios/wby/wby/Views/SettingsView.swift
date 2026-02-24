import SwiftUI

struct SettingsView: View {
    @AppStorage("dynamicEffectsEnabled") private var dynamicEffectsEnabled = true

    var body: some View {
        Form {
            Section {
                Toggle("Dynamic Background Effects", isOn: $dynamicEffectsEnabled)
            } footer: {
                Text("Shows rain, snow, and other animated weather effects in the background.")
            }
        }
        .navigationTitle("Settings")
        .navigationBarTitleDisplayMode(.large)
    }
}

#Preview {
    NavigationStack {
        SettingsView()
    }
}
