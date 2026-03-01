import CoreLocation
import SwiftUI

struct ContentView: View {
    @State private var locationService = LocationService()
    @State private var weatherService = WeatherService()
    @State private var favoritesStore = FavoritesStore()
    @State private var currentPage: Int = 0
    @State private var showingLocations = false
    private let disableAutoLoad: Bool

    private var pages: [WeatherLocation] {
        [.gps] + favoritesStore.favorites.map { .favorite($0) }
    }

    init(disableAutoLoad: Bool = false) {
        self.disableAutoLoad = disableAutoLoad
    }

    var body: some View {
        TabView(selection: $currentPage) {
            ForEach(pages, id: \.self) { location in
                let idx = pages.firstIndex(of: location) ?? 0
                WeatherPageView(
                    location: location,
                    locationService: locationService,
                    weatherService: weatherService,
                    disableAutoLoad: disableAutoLoad,
                    showingLocations: $showingLocations
                )
                .tag(idx)
            }
        }
        .tabViewStyle(.page(indexDisplayMode: .never))
        .ignoresSafeArea()
        .overlay(alignment: .bottom) {
            pageIndicator
                .padding(.bottom, 4)
        }
        .onChange(of: pages.count) {
            if currentPage >= pages.count {
                currentPage = max(0, pages.count - 1)
            }
        }
        .sheet(isPresented: $showingLocations) {
            LocationsListView(
                favoritesStore: favoritesStore,
                weatherService: weatherService,
                currentLocationName: locationService.placeName,
                currentCoordinate: locationService.coordinate
            ) { selected in
                if let selected {
                    let idx = favoritesStore.favorites.firstIndex(where: { $0.id == selected.id })
                    currentPage = (idx ?? -1) + 1
                } else {
                    currentPage = 0
                }
            }
        }
    }

    private var pageIndicator: some View {
        HStack(spacing: 8) {
            ForEach(pages, id: \.self) { location in
                let index = pages.firstIndex(of: location) ?? 0
                if case .gps = location {
                    Image(systemName: "location.fill")
                        .font(.system(size: 8))
                        .foregroundStyle(index == currentPage ? AnyShapeStyle(.primary) : AnyShapeStyle(.tertiary))
                } else {
                    Image(systemName: "circle.fill")
                        .font(.system(size: 7))
                        .foregroundStyle(index == currentPage ? AnyShapeStyle(.primary) : AnyShapeStyle(.tertiary))
                }
            }
        }
        .padding(.horizontal, 22)
        .padding(.vertical, 18)
        .glassEffect()
    }
}

#Preview {
    ContentView(disableAutoLoad: true)
}
