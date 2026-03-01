# Swipeable Locations Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the single-location weather view with a horizontally swipeable `TabView` pager — GPS always leftmost, one page per favorite — each page owning its own weather state.

**Architecture:** `ContentView` becomes a thin shell with a `TabView(.page)`. All weather content moves into a new `WeatherPageView`. Each page loads cache-first then silently refreshes in background. `LocationsListView` loses the `activeWeather`/`selectedFavoriteId` params now that each page is self-sufficient.

**Tech Stack:** SwiftUI `TabView` with `.page` style, existing `WeatherService` actor, existing `FavoritesStore`.

---

### Task 1: Create `WeatherLocation` enum

**Files:**
- Create: `ios/wby/wby/Models/WeatherLocation.swift`

**Step 1: Create the file**

```swift
import CoreLocation

enum WeatherLocation {
    case gps
    case favorite(FavoriteLocation)
}
```

**Step 2: Build**

Cmd+B in Xcode. Expected: success.

**Step 3: Commit**

```bash
git add ios/wby/wby/Models/WeatherLocation.swift
git commit -m "ios: add WeatherLocation enum"
```

---

### Task 2: Create `WeatherPageView`

**Files:**
- Create: `ios/wby/wby/Views/WeatherPageView.swift`
- Reference: `ios/wby/wby/ContentView.swift` (copy most content from here)

This is the bulk of the refactor. `WeatherPageView` is the current `ContentView` body, stripped of `NavigationStack` and toolbar, with per-page weather state and coordinate resolution.

**Step 1: Create `WeatherPageView.swift`**

```swift
import CoreLocation
import SwiftUI

struct WeatherPageView: View {
    let location: WeatherLocation
    let locationService: LocationService
    let weatherService: WeatherService
    let disableAutoLoad: Bool

    @State private var weather: WeatherResponse?
    @State private var isLoading = false
    @State private var lastUpdated: Date?
    @State private var errorMessage: String?
    @AppStorage("dynamicEffectsEnabled") private var dynamicEffectsEnabled = true

    private let fallbackCoordinate = CLLocationCoordinate2D(latitude: 60.1699, longitude: 24.9384)

    init(location: WeatherLocation, locationService: LocationService, weatherService: WeatherService, disableAutoLoad: Bool = false) {
        self.location = location
        self.locationService = locationService
        self.weatherService = weatherService
        self.disableAutoLoad = disableAutoLoad
    }

    // MARK: - Computed coordinate/name/elevation

    private var coordinate: CLLocationCoordinate2D {
        switch location {
        case .gps: return locationService.coordinate ?? fallbackCoordinate
        case .favorite(let f): return CLLocationCoordinate2D(latitude: f.latitude, longitude: f.longitude)
        }
    }

    private var locationName: String? {
        switch location {
        case .gps: return locationService.placeName
        case .favorite(let f): return f.name
        }
    }

    private var elevationMeters: Double? {
        switch location {
        case .gps: return locationService.altitudeMeters
        case .favorite: return nil
        }
    }

    // MARK: - Scene

    private var currentScene: WeatherScene {
        let symbol = weather?.hourlyForecast.first?.symbol ?? weather?.dailyForecast.first?.symbol
        return WeatherScene.from(symbolCode: nightAdjusted(symbol))
    }

    private func nightAdjusted(_ symbolCode: String?) -> String? {
        guard let code = symbolCode.flatMap(Int.init), code < 100 else { return symbolCode }
        let isNight = SunriseCard.isNight(
            coordinate: coordinate,
            date: .now,
            elevationMeters: elevationMeters ?? 0
        )
        return isNight ? String(code + 100) : symbolCode
    }

    // MARK: - Body

    var body: some View {
        ZStack {
            mainBackground
            ScrollView {
                VStack(spacing: 8) {
                    if let weather {
                        headerSection(weather)
                        if !weather.hourlyForecast.isEmpty {
                            HourlyForecastCard(
                                hourly: weather.hourlyForecast,
                                coordinate: coordinate,
                                elevationMeters: elevationMeters ?? 0
                            )
                        }
                        CurrentConditionsCard(current: weather.current)
                        dailyForecastSection(weather.dailyForecast)
                        HStack(alignment: .top, spacing: 12) {
                            FeelsLikeCard(current: weather.current)
                            UVIndexCard(
                                uvIndex: weather.hourlyForecast.compactMap(\.uvCumulated).first
                                    ?? weather.dailyForecast.compactMap(\.uvIndexAvg).first,
                                radiationGlobal: weather.current.resolvedRadiationGlobal
                                    ?? dailyResolvedRadiationGlobal(weather.dailyForecast)
                            )
                        }
                        WindCard(current: weather.current)
                        HStack(alignment: .top, spacing: 12) {
                            SunriseCard(
                                coordinate: coordinate,
                                referenceDate: weather.current.observedAt,
                                elevationMeters: elevationMeters
                            )
                            PrecipitationCard(forecasts: weather.dailyForecast)
                        }
                        HStack(alignment: .top, spacing: 12) {
                            VisibilityCard(current: weather.current)
                            HumidityCard(current: weather.current)
                        }
                        MoonPhaseCard(
                            coordinate: coordinate,
                            referenceDate: weather.current.observedAt
                        )
                        if let lastUpdated {
                            Text("Updated \(lastUpdated, style: .relative) ago")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    } else if isLoading {
                        ProgressView("Loading weather...")
                            .tint(.primary)
                            .foregroundStyle(.primary)
                            .padding(.top, 100)
                    } else {
                        ContentUnavailableView(
                            "No Weather Data",
                            systemImage: "cloud",
                            description: Text(errorMessage ?? "Pull down to refresh")
                        )
                    }
                }
                .padding()
            }
            .scrollBounceBehavior(.always)
            .refreshable { await fetchWeather() }
        }
        .task {
            guard !disableAutoLoad else { return }
            await loadWeather()
        }
        .onChange(of: locationService.coordinate?.latitude) {
            guard case .gps = location, !disableAutoLoad else { return }
            Task { await loadWeather() }
        }
    }

    // MARK: - Header

    private func headerSection(_ weather: WeatherResponse) -> some View {
        let primary: Color = currentScene.prefersLightForeground ? .white : .primary
        let secondary: Color = currentScene.prefersLightForeground ? .white.opacity(0.78) : .secondary
        return VStack(spacing: 4) {
            HStack(spacing: 4) {
                if case .gps = location {
                    Image(systemName: "location.fill")
                        .font(.subheadline)
                        .foregroundStyle(primary)
                }
                Text(locationName ?? weather.station.name)
                    .font(.title2)
                    .foregroundStyle(primary)
            }
            if let temp = weather.current.resolvedTemperature {
                Text("\(Int(temp.rounded()))°")
                    .font(.system(size: 92, weight: .light))
                    .foregroundStyle(primary)
            }
            if let feelsLike = weather.current.resolvedFeelsLike {
                Text("Feels like \(Int(feelsLike.rounded()))°")
                    .font(.subheadline)
                    .foregroundStyle(secondary)
            }
        }
        .padding(.top, 20)
    }

    // MARK: - Daily section

    private func dailyForecastSection(_ forecasts: [DailyForecast]) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            Label("\(forecasts.count - 1)-DAY FORECAST", systemImage: "calendar")
                .font(.caption)
                .foregroundStyle(.secondary)
                .padding(.bottom, 8)
            ForEach(forecasts) { day in
                DailyForecastRow(
                    forecast: day,
                    overallLow: forecasts.compactMap(\.low).min() ?? 0,
                    overallHigh: forecasts.compactMap(\.high).max() ?? 0
                )
                if day.id != forecasts.last?.id {
                    Divider().overlay(Color.primary.opacity(0.18))
                }
            }
        }
        .weatherCard()
    }

    // MARK: - Background

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
                    precipitation1h: weather?.hourlyForecast.first?.precipitation1h
                )
                .ignoresSafeArea()
            }
        }
        .animation(.easeInOut(duration: 1.5), value: currentScene)
    }

    // MARK: - Loading

    private func loadWeather() async {
        // Show cache immediately
        if let cached = await weatherService.loadFromCache(lat: coordinate.latitude, lon: coordinate.longitude) {
            weather = cached
        }
        // Then refresh in background
        await fetchWeather()
    }

    private func fetchWeather() async {
        isLoading = weather == nil
        defer { isLoading = false }
        do {
            let response = try await weatherService.fetchWeather(lat: coordinate.latitude, lon: coordinate.longitude)
            weather = response
            lastUpdated = Date()
            errorMessage = nil
            await weatherService.saveToCache(response, lat: coordinate.latitude, lon: coordinate.longitude)
        } catch {
            if weather == nil {
                errorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
            }
        }
    }

    private func dailyResolvedRadiationGlobal(_ daily: [DailyForecast]) -> Double? {
        daily.compactMap(\.radiationGlobalAvg).first.map { max(0, $0) }
    }
}
```

Note: the GPS page header adds a `location.fill` icon next to the name to distinguish it from favorites.

**Step 2: Build**

Cmd+B. Expected: success. Fix any minor issues (e.g. if `SunriseCard` init differs slightly).

**Step 3: Commit**

```bash
git add ios/wby/wby/Views/WeatherPageView.swift
git commit -m "ios: add WeatherPageView with per-page weather state"
```

---

### Task 3: Refactor `ContentView` into a paging shell

**Files:**
- Modify: `ios/wby/wby/ContentView.swift`

Replace the entire file content. `ContentView` keeps `NavigationStack` (so `NavigationLink` to `SettingsView` still works), wraps a `TabView`, and adds a page indicator overlay.

```swift
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
```

Note: the old `previewWeather` init param is removed since `WeatherPageView` handles its own state. The preview will show a loading state — acceptable.

**Step 2: Build**

Cmd+B. Expected: errors about `LocationsListView` still expecting `activeWeather` and `selectedFavoriteId`. Fix in Task 4.

**Step 3: (Do not commit yet — wait for Task 4)**

---

### Task 4: Remove stale params from `LocationsListView`

**Files:**
- Modify: `ios/wby/wby/Views/LocationsListView.swift`

**Step 1: Remove `activeWeather` and `selectedFavoriteId` from the struct**

Remove these two properties:
```swift
let activeWeather: WeatherResponse?
let selectedFavoriteId: UUID?
```

**Step 2: Remove the seeding block from `loadWeathers()`**

Remove:
```swift
// Seed active location's weather immediately from already-loaded data
if let active = activeWeather {
    if let id = selectedFavoriteId {
        favoriteWeathers[id] = active
    } else {
        myLocationWeather = active
    }
}
```

**Step 3: Update the Preview at the bottom of the file**

Remove `activeWeather:` and `selectedFavoriteId:` from the `LocationsListView(...)` call in `#Preview`.

**Step 4: Build**

Cmd+B. Expected: success.

**Step 5: Commit**

```bash
git add ios/wby/wby/ContentView.swift ios/wby/wby/Views/LocationsListView.swift ios/wby/wby/Views/WeatherPageView.swift
git commit -m "ios: swipeable location pages with TabView pager"
```

---

### Task 5: Verify the feature end-to-end

**Step 1: Run on simulator**

Build and run on iPhone 16 simulator (iOS 18+).

**Checklist:**
- [ ] App launches showing GPS page (leftmost)
- [ ] Adding a favorite in the locations list creates a new page to the right
- [ ] Swiping left moves to favorite pages; swiping right goes back to GPS
- [ ] Each page loads its own weather (cache first, then refreshes)
- [ ] GPS page header shows `location.fill` icon; favorite pages do not
- [ ] Tapping a location in the list sheet jumps to the correct page
- [ ] Tapping GPS in the list sheet jumps to page 0
- [ ] Swipe-to-delete a favorite in the list sheet → that page disappears
- [ ] Page indicator dots update as you swipe; GPS slot shows pin icon
- [ ] Pull-to-refresh works on each page independently
- [ ] Settings button still navigates to SettingsView

**Step 2: Fix any issues found, commit fixes individually**
