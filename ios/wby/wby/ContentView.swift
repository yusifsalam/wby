import CoreLocation
import SwiftUI

struct ContentView: View {
    private enum PageID: Hashable {
        case gps
        case favorite(UUID)
    }

    private struct Page: Identifiable, Hashable {
        let id: PageID
        let location: WeatherLocation
    }

    private struct PageBackgroundState: Equatable {
        let scene: WeatherScene
        let precipitation1h: Double?
    }

    @State private var locationService = LocationService()
    @State private var weatherService = WeatherService()
    @State private var favoritesStore = FavoritesStore()
    @State private var currentPageID: PageID = .gps
    @State private var showingLocations = false
    @State private var showingSettings = false
    @State private var showingMap = false
    @State private var pendingPageID: PageID? = nil
    @State private var pageBackgrounds: [PageID: PageBackgroundState] = [:]
    @AppStorage("dynamicEffectsEnabled") private var dynamicEffectsEnabled = true
    private let disableAutoLoad: Bool
    private let initialWeather: WeatherResponse?

    private var pages: [Page] {
        [Page(id: .gps, location: .gps)] + favoritesStore.favorites.map { favorite in
            Page(id: .favorite(favorite.id), location: .favorite(favorite))
        }
    }

    private var pageIDs: Set<PageID> {
        Set(pages.map(\.id))
    }

    private var activeBackground: PageBackgroundState {
        pageBackgrounds[currentPageID] ?? PageBackgroundState(scene: .clearDay, precipitation1h: nil)
    }

    init(disableAutoLoad: Bool = false, initialWeather: WeatherResponse? = nil) {
        self.disableAutoLoad = disableAutoLoad
        self.initialWeather = initialWeather
    }

    var body: some View {
        NavigationStack {
            ZStack {
                rootBackground

                ScrollView(.horizontal) {
                    LazyHStack(spacing: 0) {
                        ForEach(pages) { page in
                            WeatherPageView(
                                location: page.location,
                                locationService: locationService,
                                weatherService: weatherService,
                                disableAutoLoad: disableAutoLoad,
                                initialWeather: initialWeather,
                                onBackgroundUpdate: { scene, precipitation in
                                    pageBackgrounds[page.id] = PageBackgroundState(
                                        scene: scene,
                                        precipitation1h: precipitation
                                    )
                                }
                            )
                            .containerRelativeFrame(.horizontal)
                            .id(page.id)
                        }
                    }
                    .scrollTargetLayout()
                }
                .scrollTargetBehavior(.paging)
                .scrollPosition(id: Binding<PageID?>(
                    get: { currentPageID },
                    set: { if let id = $0 { currentPageID = id } }
                ))
                .scrollIndicators(.hidden)
                .overlay(alignment: .bottom) {
                    pageIndicator
                        .padding(.bottom, 4)
                }
            }
            .onChange(of: pages.map(\.id)) {
                pageBackgrounds = pageBackgrounds.filter { pageIDs.contains($0.key) }
                if !pageIDs.contains(currentPageID) {
                    currentPageID = .gps
                }
            }
            .sheet(isPresented: $showingLocations, onDismiss: {
                if let pageID = pendingPageID {
                    currentPageID = pageIDs.contains(pageID) ? pageID : .gps
                    pendingPageID = nil
                }
            }) {
                LocationsListView(
                    favoritesStore: favoritesStore,
                    weatherService: weatherService,
                    currentLocationName: locationService.placeName,
                    currentCoordinate: locationService.coordinate
                ) { selected in
                    if let selected {
                        pendingPageID = .favorite(selected.id)
                    } else {
                        pendingPageID = .gps
                    }
                }
            }
            .sheet(isPresented: $showingSettings) {
                NavigationStack {
                    SettingsView()
                }
            }
            .sheet(isPresented: $showingMap) {
                NavigationStack {
                    WeatherMapView(
                        locationService: locationService,
                        favoritesStore: favoritesStore,
                        weatherService: weatherService
                    )
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
                    Button { showingMap = true } label: {
                        Image(systemName: "map")
                            .foregroundStyle(.white)
                            .accessibilityLabel("Map")
                    }
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button { showingSettings = true } label: {
                        Image(systemName: "gearshape")
                            .foregroundStyle(.white)
                            .accessibilityLabel("Settings")
                    }
                }
            }
            .navigationTitle("")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar(.visible, for: .navigationBar)
            .toolbarBackground(.hidden, for: .navigationBar)
        }
    }

    private var pageIndicator: some View {
        HStack(spacing: 8) {
            ForEach(pages) { page in
                let isActive = page.id == currentPageID
                if case .gps = page.location {
                    Image(systemName: "location.fill")
                        .font(.system(size: 8))
                        .foregroundStyle(isActive ? AnyShapeStyle(.primary) : AnyShapeStyle(.tertiary))
                } else {
                    Image(systemName: "circle.fill")
                        .font(.system(size: 7))
                        .foregroundStyle(isActive ? AnyShapeStyle(.primary) : AnyShapeStyle(.tertiary))
                }
            }
        }
        .padding(.horizontal, 22)
        .padding(.vertical, 18)
        .glassEffect()
    }

    private var rootBackground: some View {
        ZStack {
            LinearGradient(
                colors: activeBackground.scene.gradientColors,
                startPoint: .top,
                endPoint: .bottom
            )
            .ignoresSafeArea()
            .id(activeBackground.scene)
            .transition(.opacity)

            if dynamicEffectsEnabled {
                WeatherSceneView(
                    weatherScene: activeBackground.scene,
                    precipitation1h: activeBackground.precipitation1h
                )
                .ignoresSafeArea()
            }
        }
        .animation(.easeInOut(duration: 1.5), value: activeBackground.scene)
    }
}

#Preview {
    ContentView(disableAutoLoad: true, initialWeather: PreviewData.makeSample())
}
