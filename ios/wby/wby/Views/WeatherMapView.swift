import CoreLocation
import SwiftUI

struct WeatherMapView: View {
    let locationService: LocationService
    let favoritesStore: FavoritesStore
    private let overlayTimeZone = TimeZone(identifier: "Europe/Helsinki")!

    @Environment(\.dismiss) private var dismiss
    @StateObject private var viewModel: WeatherMapViewModel

    init(
        locationService: LocationService,
        favoritesStore: FavoritesStore,
        weatherService: WeatherService
    ) {
        self.locationService = locationService
        self.favoritesStore = favoritesStore
        _viewModel = StateObject(
            wrappedValue: WeatherMapViewModel(
                overlayService: MapOverlayService(weatherService: weatherService),
                weatherService: weatherService
            )
        )
    }

    var body: some View {
        ZStack {
            WeatherMapUIKitBridge(viewModel: viewModel)
                .ignoresSafeArea()

            VStack {
                HStack(alignment: .top) {
                    VStack(alignment: .leading, spacing: 10) {
                        Button {
                            dismiss()
                        } label: {
                            Image(systemName: "xmark")
                                .font(.system(size: 14, weight: .bold))
                                .frame(width: 36, height: 36)
                                .background(.ultraThinMaterial, in: Circle())
                        }
                        .buttonStyle(.plain)
                        .accessibilityLabel("Close map")

                        TemperatureLegendView()
                    }
                    Spacer()
                    if let meta = viewModel.meta {
                        VStack(alignment: .trailing, spacing: 4) {
                            if let min = meta.minTemp, let max = meta.maxTemp {
                                Text("\(Int(min.rounded()))° ... \(Int(max.rounded()))°")
                                    .font(.caption2.bold())
                            }
                            if let dataTime = meta.dataTime {
                                Text(formatOverlayDataTime(dataTime))
                                    .font(.caption2)
                                    .foregroundStyle(.secondary)
                            }
                        }
                        .padding(10)
                        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 10))
                    }
                }
                Spacer()
            }
            .padding()
        }
        .task {
            viewModel.setPreferredCenter(locationService.coordinate)
            viewModel.setFavoriteLocations(favoritesStore.favorites)
            _ = await locationService.requestFreshLocation()
        }
        .onChange(of: locationService.coordinate.map { "\($0.latitude),\($0.longitude)" }) {
            viewModel.setPreferredCenter(locationService.coordinate)
        }
        .onChange(of: favoritesStore.favorites) {
            viewModel.setFavoriteLocations(favoritesStore.favorites)
        }
    }

    private func formatOverlayDataTime(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.dateStyle = .none
        formatter.timeStyle = .short
        formatter.timeZone = overlayTimeZone
        return formatter.string(from: date)
    }
}
