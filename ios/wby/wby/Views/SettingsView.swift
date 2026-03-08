import SwiftUI

struct SettingsView: View {
    @AppStorage("dynamicEffectsEnabled") private var dynamicEffectsEnabled = true
    @State private var showingPurgeConfirmation = false
    @State private var cachePurged = false

    var body: some View {
        Form {
            Section {
                Toggle("Dynamic Background Effects", isOn: $dynamicEffectsEnabled)
            } footer: {
                Text("Shows rain, snow, and other animated weather effects in the background.")
            }

            Section {
                Button(role: .destructive) {
                    showingPurgeConfirmation = true
                } label: {
                    Text(cachePurged ? "Cache Cleared" : "Clear Cache")
                }
                .disabled(cachePurged)
                .confirmationDialog("Clear all cached weather data?", isPresented: $showingPurgeConfirmation, titleVisibility: .visible) {
                    Button("Clear Cache", role: .destructive) {
                        URLCache.shared.removeAllCachedResponses()
                        cachePurged = true
                    }
                }
            } footer: {
                Text("Removes cached weather and climate data. Fresh data will be fetched on next load.")
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
