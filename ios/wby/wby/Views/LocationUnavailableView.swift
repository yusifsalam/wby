import SwiftUI
import UIKit

struct LocationUnavailableView: View {
    let isDenied: Bool
    let didFail: Bool

    @Environment(\.openURL) private var openURL

    var body: some View {
        VStack {
            if isDenied {
                ContentUnavailableView {
                    Label("Location Access Off", systemImage: "location.slash")
                } description: {
                    Text("Allow location access in Settings to see weather here.")
                } actions: {
                    Button("Open Settings") {
                        if let url = URL(string: UIApplication.openSettingsURLString) {
                            openURL(url)
                        }
                    }
                }
            } else if didFail {
                ContentUnavailableView(
                    "Location Unavailable",
                    systemImage: "location.slash",
                    description: Text("Pull down to try again.")
                )
            } else {
                ProgressView("Finding your location...")
                    .tint(.primary)
                    .foregroundStyle(.primary)
                    .padding(.top, 100)
            }
        }
    }
}

#Preview("Locating") {
    LocationUnavailableView(isDenied: false, didFail: false)
}

#Preview("Denied") {
    LocationUnavailableView(isDenied: true, didFail: false)
}

#Preview("Failed") {
    LocationUnavailableView(isDenied: false, didFail: true)
}
