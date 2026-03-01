# Swipeable Locations Design

**Date:** 2026-03-01

## Goal

Replace the single-location weather view with a horizontally swipeable pager — one page per saved location, GPS always leftmost — matching the Apple Weather interaction model.

## Architecture

`ContentView` becomes a thin shell owning shared services and page state. All weather content moves into `WeatherPageView`.

### Location enum

```swift
enum WeatherLocation {
    case gps
    case favorite(FavoriteLocation)
}
```

Pages array: `[.gps] + favoritesStore.favorites.map { .favorite($0) }`

### ContentView responsibilities

- Owns: `locationService`, `weatherService`, `favoritesStore`, `currentPage: Int = 0`
- Renders `TabView(selection: $currentPage)` with `.tabViewStyle(.page(indexDisplayMode: .never))`
- Overlays a custom page indicator at the bottom
- Shows locations list sheet; tapping a location sets `currentPage` and dismisses

### WeatherPageView

Each page is a self-contained view that owns its own weather state.

**Inputs:** `location: WeatherLocation`, `locationService: LocationService`, `weatherService: WeatherService`

**State:** `weather: WeatherResponse?`, `isLoading: Bool`, `lastUpdated: Date?`, `errorMessage: String?`

**Layout:** `ZStack` with background (gradient + particles) behind a vertical `ScrollView` of weather cards. Background lives inside the page so it slides with swipe.

**Loading:** On `.task` — load from cache immediately, then fetch fresh in background. GPS page also reloads on `.onChange(of: locationService.coordinate)`.

**Coordinates:**
- GPS page: `locationService.coordinate ?? fallbackCoordinate`, `locationService.altitudeMeters`
- Favorite page: `CLLocationCoordinate2D(latitude: f.latitude, longitude: f.longitude)`, `elevationMeters: 0`

### Page indicator

Custom `HStack` overlaid at screen bottom (above home indicator):
- GPS slot: `Image(systemName: "location.fill")`
- Favorite slots: circle dots
- Current page: white, others: white @ 40% opacity

### LocationsListView changes

`onSelect` callback returns `Int` (page index) instead of `FavoriteLocation?`. Tapping GPS → index 0, tapping favorite → its index in the pages array.

## What's removed from ContentView

- `selectedFavorite: FavoriteLocation?`
- `weather`, `isLoading`, `lastUpdated`, `errorMessage`
- `loadWeather()`, `fetchWeather()`
