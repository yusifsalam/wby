import SwiftUI

struct SettingsView: View {
    @AppStorage("dynamicEffectsEnabled") private var dynamicEffectsEnabled = true
    @AppStorage(OverlayMode.storageKey) private var overlayModeRawValue = OverlayMode.metal.rawValue
    @AppStorage(PrecipRampStyle.storageKey) private var precipRampRawValue = PrecipRampStyle.smooth.rawValue
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
                Toggle("Use Metal Renderer", isOn: usesMetalRenderer)
                Picker("Rain Colors", selection: $precipRampRawValue) {
                    ForEach(PrecipRampStyle.allCases, id: \.rawValue) { style in
                        Text(style.displayName).tag(style.rawValue)
                    }
                }
            } header: {
                Text("Map weather overlay")
            } footer: {
                Text("When off, the app uses the PNG overlay backend. Stepped rain colors match the radar classes used by FMI.")
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

    private var usesMetalRenderer: Binding<Bool> {
        Binding(
            get: { (OverlayMode(rawValue: overlayModeRawValue) ?? .metal) == .metal },
            set: { overlayModeRawValue = $0 ? OverlayMode.metal.rawValue : OverlayMode.png.rawValue }
        )
    }
}

#Preview {
    NavigationStack {
        SettingsView()
    }
}
