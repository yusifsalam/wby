import CoreLocation
import SwiftUI

private enum PageID: Hashable {
    case gps
    case favorite(UUID)
}

/// Holds per-page scroll offsets, which update on every scroll frame. Keeping
/// this in an `@Observable` model (rather than `@State` on `ContentView`) means
/// only the views that read an offset re-evaluate when it changes — the paging
/// `ScrollView` and its page subtree stay stable during scrolling.
@MainActor
@Observable
private final class PageScrollModel {
    var offsets: [PageID: CGFloat] = [:]
}

struct ContentView: View {
    private struct Page: Identifiable, Hashable {
        let id: PageID
        let location: WeatherLocation
    }

    private struct PageBackgroundState: Equatable {
        let scene: WeatherScene
        let precipitation1h: Double?
        let cloudCover: Double?
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
    @State private var scrollModel = PageScrollModel()
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
        pageBackgrounds[currentPageID] ?? PageBackgroundState(scene: .clearDay, precipitation1h: nil, cloudCover: nil)
    }

    init(disableAutoLoad: Bool = false, initialWeather: WeatherResponse? = nil) {
        self.disableAutoLoad = disableAutoLoad
        self.initialWeather = initialWeather
    }

    var body: some View {
        NavigationStack {
            ZStack {
                RootBackgroundView(
                    scene: activeBackground.scene,
                    precipitation1h: activeBackground.precipitation1h,
                    cloudCover: activeBackground.cloudCover,
                    dynamicEffectsEnabled: dynamicEffectsEnabled,
                    currentPageID: currentPageID,
                    scrollModel: scrollModel
                )
                .zIndex(0)

                ScrollView(.horizontal) {
                    LazyHStack(spacing: 0) {
                        ForEach(pages) { page in
                            WeatherPageView(
                                location: page.location,
                                locationService: locationService,
                                weatherService: weatherService,
                                disableAutoLoad: disableAutoLoad,
                                initialWeather: initialWeather,
                                onBackgroundUpdate: { scene, precipitation, cloudCover in
                                    pageBackgrounds[page.id] = PageBackgroundState(
                                        scene: scene,
                                        precipitation1h: precipitation,
                                        cloudCover: cloudCover
                                    )
                                },
                                onScrollOffsetChange: { offset in
                                    scrollModel.offsets[page.id] = offset
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
                .zIndex(1)

                VStack {
                    Spacer()
                    bottomBar
                        .padding(.bottom, 4)
                }
                .zIndex(2)
            }
            .onChange(of: pages.map(\.id)) {
                pageBackgrounds = pageBackgrounds.filter { pageIDs.contains($0.key) }
                scrollModel.offsets = scrollModel.offsets.filter { pageIDs.contains($0.key) }
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
                    currentCoordinate: locationService.coordinate,
                    onSelect: { selected in
                        if let selected {
                            pendingPageID = .favorite(selected.id)
                        } else {
                            pendingPageID = .gps
                        }
                    },
                    disableAutoLoad: disableAutoLoad
                )
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
                    weatherService: weatherService,
                    disableAutoLoad: disableAutoLoad
                )
            }
            .fullScreenCover(isPresented: $showingLeaderboard) {
                LeaderboardView(
                    locationService: locationService,
                    weatherService: weatherService,
                    disableAutoLoad: disableAutoLoad
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

}

/// Renders the shared backdrop behind the paging weather views. It owns the
/// read of the high-frequency scroll offset (via `scrollModel`), so the
/// scroll-driven sun fade re-evaluates only this view rather than the whole
/// `ContentView` body.
private struct RootBackgroundView: View {
    let scene: WeatherScene
    let precipitation1h: Double?
    let cloudCover: Double?
    let dynamicEffectsEnabled: Bool
    let currentPageID: PageID
    let scrollModel: PageScrollModel

    private var sunOpacity: Double {
        let distance = max(0, scrollModel.offsets[currentPageID] ?? 0)
        let fadeStart: CGFloat = 0
        let fadeEnd: CGFloat = 140
        if distance <= fadeStart { return 1 }
        if distance >= fadeEnd { return 0 }
        return Double(1 - (distance - fadeStart) / (fadeEnd - fadeStart))
    }

    var body: some View {
        ZStack {
            LinearGradient(
                colors: scene.gradientColors,
                startPoint: .top,
                endPoint: .bottom
            )
            .ignoresSafeArea()
            .id(scene)
            .transition(.opacity)

            if dynamicEffectsEnabled {
                WeatherBackgroundView(
                    weatherScene: scene,
                    precipitation1h: precipitation1h,
                    cloudCover: cloudCover,
                    sunOpacity: sunOpacity
                )
                .ignoresSafeArea()
            }
        }
        .animation(.easeInOut(duration: 1.5), value: scene)
    }
}

#Preview {
    ContentView(disableAutoLoad: true, initialWeather: PreviewData.makeSample())
}
