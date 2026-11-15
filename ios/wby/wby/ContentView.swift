import CoreLocation
import SwiftUI

struct ContentView: View {
    private struct Page: Identifiable, Hashable {
        let id: WeatherPageID
        let location: WeatherLocation
    }

    @State private var locationService = LocationService()
    @State private var weatherStore = WeatherStore()
    @State private var favoritesStore = FavoritesStore()
    @State private var currentPageID: WeatherPageID = .gps
    @State private var showingLocations = false
    @State private var showingSettings = false
    @State private var showingLeaderboard = false
    @State private var pendingPageID: WeatherPageID? = nil
    @State private var pageSheetPresented = false
    @State private var pager = WeatherPagerModel()
    @AppStorage("dynamicEffectsEnabled") private var dynamicEffectsEnabled = true
    private let disableAutoLoad: Bool

    private var pages: [Page] {
        [Page(id: .gps, location: .gps)] + favoritesStore.favorites.map { favorite in
            Page(id: .favorite(favorite.id), location: .favorite(favorite))
        }
    }

    private var pageIDs: Set<WeatherPageID> {
        Set(pages.map(\.id))
    }

    init(disableAutoLoad: Bool = false) {
        self.disableAutoLoad = disableAutoLoad
    }

    var body: some View {
        NavigationStack {
            ZStack {
                RootBackgroundView(
                    dynamicEffectsEnabled: dynamicEffectsEnabled,
                    isCovered: showingLeaderboard || showingLocations || showingSettings || pageSheetPresented,
                    currentPageID: currentPageID,
                    pager: pager
                )
                .zIndex(0)

                ScrollView(.horizontal) {
                    LazyHStack(spacing: 0) {
                        ForEach(pages) { page in
                            WeatherPageView(
                                location: page.location,
                                pageID: page.id,
                                pager: pager,
                                locationService: locationService,
                                weatherStore: weatherStore,
                                disableAutoLoad: disableAutoLoad
                            )
                            .containerRelativeFrame(.horizontal)
                            .id(page.id)
                        }
                    }
                    .scrollTargetLayout()
                }
                .scrollTargetBehavior(.paging)
                .scrollPosition(id: Binding<WeatherPageID?>(
                    get: { currentPageID },
                    set: { if let id = $0 { currentPageID = id } }
                ))
                .scrollIndicators(.hidden)
                .onPreferenceChange(BackgroundCoveredKey.self) { pageSheetPresented = $0 }
                .safeAreaBar(edge: .bottom) {
                    bottomBar
                }
                .zIndex(1)
            }
            .onChange(of: pages.map(\.id)) {
                pager.backgrounds = pager.backgrounds.filter { pageIDs.contains($0.key) }
                pager.offsets = pager.offsets.filter { pageIDs.contains($0.key) }
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
                    weatherStore: weatherStore,
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
            .fullScreenCover(isPresented: $pager.showingMap) {
                WeatherMapView(
                    locationService: locationService,
                    favoritesStore: favoritesStore,
                    weatherStore: weatherStore,
                    disableAutoLoad: disableAutoLoad
                )
            }
            .fullScreenCover(isPresented: $showingLeaderboard) {
                LeaderboardView(
                    locationService: locationService,
                    weatherService: weatherStore.service,
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
            .toolbarVisibility(.visible, for: .navigationBar)
            .toolbarBackgroundVisibility(.hidden, for: .navigationBar)
        }
    }

    private var bottomBar: some View {
        GlassEffectContainer(spacing: 20) {
            HStack {
                circleButton("Map", icon: "map") { pager.showingMap = true }

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
/// reads of the pager model (the current page's background, its
/// high-frequency scroll offset, and whether the map covers the pages), so
/// those changes re-evaluate only this view rather than `ContentView`.
private struct RootBackgroundView: View {
    let dynamicEffectsEnabled: Bool
    let isCovered: Bool
    let currentPageID: WeatherPageID
    let pager: WeatherPagerModel

    private var sunFade: Double {
        let distance = max(0, pager.offsets[currentPageID] ?? 0)
        let fadeEnd: CGFloat = 140
        let fade = 1 - min(distance / fadeEnd, 1)
        return (Double(fade) * 12).rounded() / 12
    }

    var body: some View {
        let state = pager.backgrounds[currentPageID]
        let gradientScene = state?.scene ?? .clearDay
        ZStack {
            LinearGradient(
                colors: gradientScene.gradientColors,
                startPoint: .top,
                endPoint: .bottom
            )
            .ignoresSafeArea()
            .id(gradientScene)
            .transition(.opacity)

            if dynamicEffectsEnabled, let state {
                WeatherBackgroundView(state: state, sunFade: sunFade, isCovered: isCovered || pager.showingMap)
                    .ignoresSafeArea()
                    .transition(.opacity)
            }
        }
        .animation(.easeInOut(duration: 1.5), value: state?.scene)
    }
}

#Preview {
    ContentView(disableAutoLoad: true)
}
