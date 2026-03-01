import CoreLocation
import SwiftUI

struct ContentView: View {
    @State private var locationService = LocationService()
    @State private var weatherService = WeatherService()
    @State private var favoritesStore = FavoritesStore()
    @State private var currentPage: Int = 0
    @State private var showingLocations = false
    @State private var pageScenes: [Int: WeatherScene] = [:]
    @State private var pagePrecip: [Int: Double?] = [:]
    @AppStorage("dynamicEffectsEnabled") private var dynamicEffectsEnabled = true
    private let disableAutoLoad: Bool

    private var pages: [WeatherLocation] {
        [.gps] + favoritesStore.favorites.map { .favorite($0) }
    }

    private var currentScene: WeatherScene {
        pageScenes[currentPage] ?? WeatherScene.from(symbolCode: nil)
    }

    private var currentPrecip: Double? {
        pagePrecip[currentPage] ?? nil
    }

    init(disableAutoLoad: Bool = false) {
        self.disableAutoLoad = disableAutoLoad
    }

    var body: some View {
        NavigationStack {
            ZStack {
                mainBackground
                TabView(selection: $currentPage) {
                    ForEach(pages, id: \.self) { location in
                        let idx = pages.firstIndex(of: location) ?? 0
                        WeatherPageView(
                            location: location,
                            locationService: locationService,
                            weatherService: weatherService,
                            disableAutoLoad: disableAutoLoad,
                            pageIndex: idx,
                            onSceneChange: { index, scene, precip in
                                pageScenes[index] = scene
                                pagePrecip[index] = precip
                            }
                        )
                        .tag(idx)
                    }
                }
                .tabViewStyle(.page(indexDisplayMode: .never))
                .ignoresSafeArea(edges: .top)
                .overlay(alignment: .bottom) {
                    pageIndicator
                        .padding(.bottom, 8)
                }
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

    private var mainBackground: some View {
        ZStack {
            LinearGradient(
                colors: currentScene.gradientColors,
                startPoint: .top,
                endPoint: .bottom
            )
            .ignoresSafeArea()
            .id(currentScene)
            .transition(.opacity)

            if dynamicEffectsEnabled {
                WeatherSceneView(
                    weatherScene: currentScene,
                    precipitation1h: currentPrecip
                )
                .ignoresSafeArea()
            }
        }
        .animation(.easeInOut(duration: 1.5), value: currentScene)
    }

    private var pageIndicator: some View {
        HStack(spacing: 8) {
            ForEach(pages, id: \.self) { location in
                let index = pages.firstIndex(of: location) ?? 0
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
