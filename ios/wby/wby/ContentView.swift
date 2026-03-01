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
        NavigationStack {
            TabView(selection: $currentPage) {
                ForEach(Array(pages.enumerated()), id: \.offset) { index, location in
                    WeatherPageView(
                        location: location,
                        locationService: locationService,
                        weatherService: weatherService,
                        disableAutoLoad: disableAutoLoad
                    )
                    .tag(index)
                }
            }
            .tabViewStyle(.page(indexDisplayMode: .never))
            .ignoresSafeArea()
            .overlay(alignment: .bottom) {
                pageIndicator
                    .padding(.bottom, 8)
            }
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button { showingLocations = true } label: {
                        Image(systemName: "list.bullet")
                            .foregroundStyle(.white)
                            .accessibilityLabel("Locations")
                    }
                }
                ToolbarItem(placement: .topBarTrailing) {
                    NavigationLink {
                        SettingsView()
                    } label: {
                        Image(systemName: "gearshape")
                            .foregroundStyle(.white)
                            .accessibilityLabel("Settings")
                    }
                }
            }
            .toolbarBackground(.clear, for: .navigationBar)
            .navigationTitle("")
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
    }

    private var pageIndicator: some View {
        HStack(spacing: 8) {
            ForEach(Array(pages.enumerated()), id: \.offset) { index, location in
                if case .gps = location {
                    Image(systemName: "location.fill")
                        .font(.system(size: 8))
                        .foregroundStyle(index == currentPage ? .white : .white.opacity(0.4))
                } else {
                    Circle()
                        .fill(index == currentPage ? Color.white : Color.white.opacity(0.4))
                        .frame(width: 7, height: 7)
                }
            }
        }
    }
}

#Preview {
    ContentView(disableAutoLoad: true)
}
