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

    @State private var locationService = LocationService()
    @State private var weatherService = WeatherService()
    @State private var favoritesStore = FavoritesStore()
    @State private var currentPageID: PageID = .gps
    @State private var showingLocations = false
    @State private var showingSettings = false
    @State private var showingMap = false
    @State private var showingLeaderboard = false
    @State private var pendingPageID: PageID? = nil
    @State private var pageBackgrounds: [PageID: WeatherBackgroundState] = [:]
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

    private var activeBackground: WeatherBackgroundState {
        pageBackgrounds[currentPageID] ?? .placeholder
    }

    init(disableAutoLoad: Bool = false, initialWeather: WeatherResponse? = nil) {
        self.disableAutoLoad = disableAutoLoad
        self.initialWeather = initialWeather
    }

    var body: some View {
        NavigationStack {
            ZStack {
                RootBackgroundView(
                    state: activeBackground,
                    dynamicEffectsEnabled: dynamicEffectsEnabled,
                    isCovered: showingMap || showingLeaderboard,
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
                                onBackgroundUpdate: { state in
                                    pageBackgrounds[page.id] = state
                                },
                                onScrollOffsetChange: { offset in
                                    scrollModel.offsets[page.id] = offset
                                },
                                onOpenMap: { showingMap = true }
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
                .safeAreaBar(edge: .bottom) {
                    bottomBar
                }
                .zIndex(1)
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
                    Button("Locations", systemImage: "list.bullet") { showingLocations = true }
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button("Settings", systemImage: "gearshape") { showingSettings = true }
                }
            }
            .navigationTitle("")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar(.visible, for: .navigationBar)
            .toolbarBackground(.hidden, for: .navigationBar)
        }
    }

    private var bottomBar: some View {
        GlassEffectContainer(spacing: 20) {
            HStack {
                circleButton("Map", icon: "map") { showingMap = true }

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

                circleButton("Leaderboard", icon: "chart.bar.fill") { showingLeaderboard = true }
            }
            .padding(.horizontal, 20)
            .padding(.bottom, 4)
        }
    }

    private func circleButton(_ title: LocalizedStringKey, icon: String, action: @escaping () -> Void) -> some View {
        Button(title, systemImage: icon, action: action)
            .labelStyle(.iconOnly)
            .font(.system(size: 17, weight: .medium))
            .buttonStyle(.glass)
            .buttonBorderShape(.circle)
            .controlSize(.large)
            .tint(.primary)
    }

}

/// Renders the shared backdrop behind the paging weather views. It owns the
/// read of the high-frequency scroll offset (via `scrollModel`), so the
/// scroll-driven sun fade re-evaluates only this view rather than the whole
/// `ContentView` body.
private struct RootBackgroundView: View {
    let state: WeatherBackgroundState
    let dynamicEffectsEnabled: Bool
    let isCovered: Bool
    let currentPageID: PageID
    let scrollModel: PageScrollModel

    private var sunFade: Double {
        let distance = max(0, scrollModel.offsets[currentPageID] ?? 0)
        let fadeEnd: CGFloat = 140
        let fade = 1 - min(distance / fadeEnd, 1)
        return (Double(fade) * 12).rounded() / 12
    }

    var body: some View {
        ZStack {
            LinearGradient(
                colors: state.scene.gradientColors,
                startPoint: .top,
                endPoint: .bottom
            )
            .ignoresSafeArea()
            .id(state.scene)
            .transition(.opacity)

            if dynamicEffectsEnabled {
                WeatherBackgroundView(state: state, sunFade: sunFade, isCovered: isCovered)
                    .ignoresSafeArea()
            }
        }
        .animation(.easeInOut(duration: 1.5), value: state.scene)
    }
}

#Preview {
    ContentView(disableAutoLoad: true, initialWeather: PreviewData.makeSample())
}
