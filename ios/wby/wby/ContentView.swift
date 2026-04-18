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
    @State private var showingLeaderboard = false
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

                VStack {
                    Spacer()
                    bottomBar
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
                    let resolved = pageIDs.contains(pageID) ? pageID : .gps
                    withAnimation(.easeInOut(duration: 0.4)) {
                        currentPageID = resolved
                    }
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
            .fullScreenCover(isPresented: $showingMap) {
                WeatherMapView(
                    locationService: locationService,
                    favoritesStore: favoritesStore,
                    weatherService: weatherService
                )
            }
            .fullScreenCover(isPresented: $showingLeaderboard) {
                LeaderboardView(
                    locationService: locationService,
                    weatherService: weatherService
                )
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

    private var bottomBar: some View {
        HStack {
            circleButton(icon: "map") { showingMap = true }
                .accessibilityLabel("Map")

            Spacer()

            HStack(spacing: 6) {
                ForEach(pages) { page in
                    let isCurrent = page.id == currentPageID
                    if case .gps = page.location {
                        Image(systemName: "location.fill")
                            .font(.system(size: 8))
                            .foregroundStyle(isCurrent ? AnyShapeStyle(.primary) : AnyShapeStyle(.tertiary))
                    } else {
                        Image(systemName: "circle.fill")
                            .font(.system(size: 7))
                            .foregroundStyle(isCurrent ? AnyShapeStyle(.primary) : AnyShapeStyle(.tertiary))
                    }
                }
            }
            .padding(.horizontal, 22)
            .padding(.vertical, 18)
            .glassEffect(in: .capsule)

            Spacer()

            circleButton(icon: "chart.bar.fill") { showingLeaderboard = true }
                .accessibilityLabel("Leaderboard")
        }
        .padding(.horizontal, 20)
        .padding(.bottom, 4)
    }

    private func circleButton(icon: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Image(systemName: icon)
                .font(.system(size: 17, weight: .medium))
                .foregroundStyle(.white)
                .frame(width: 50, height: 50)
                .glassEffect(in: .circle)
        }
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
