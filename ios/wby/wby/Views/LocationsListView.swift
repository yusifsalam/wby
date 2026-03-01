import CoreLocation
import MapKit
import SwiftUI

struct LocationsListView: View {
    let favoritesStore: FavoritesStore
    let weatherService: WeatherService
    let currentLocationName: String?
    let currentCoordinate: CLLocationCoordinate2D?
    let onSelect: (FavoriteLocation?) -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var myLocationWeather: WeatherResponse?
    @State private var favoriteWeathers: [UUID: WeatherResponse] = [:]
    @State private var searchText = ""
    @State private var completer = LocationSearchCompleter()
    @State private var swipeOffsets: [UUID: CGFloat] = [:]

    var body: some View {
        NavigationStack {
            ZStack {
                Color(red: 0.07, green: 0.07, blue: 0.10)
                    .ignoresSafeArea()

                ScrollView {
                    VStack(spacing: 12) {
                        if searchText.isEmpty {
                            locationCard(
                                name: currentLocationName ?? "My Location",
                                isMyLocation: true,
                                weather: myLocationWeather
                            )
                            .onTapGesture { onSelect(nil); dismiss() }

                            ForEach(favoritesStore.favorites) { favorite in
                                let swipeOffset = swipeOffsets[favorite.id] ?? 0
                                ZStack(alignment: .trailing) {
                                    // Delete background revealed on swipe
                                    RoundedRectangle(cornerRadius: 16)
                                        .fill(.red)
                                        .overlay(alignment: .trailing) {
                                            Image(systemName: "trash")
                                                .foregroundStyle(.white)
                                                .font(.title2)
                                                .padding(.trailing, 24)
                                        }
                                        .opacity(min(1, -swipeOffset / 60))

                                    locationCard(
                                        name: favorite.name,
                                        isMyLocation: false,
                                        weather: favoriteWeathers[favorite.id]
                                    )
                                    .onTapGesture {
                                        if swipeOffset != 0 {
                                            withAnimation(.spring(response: 0.3, dampingFraction: 0.8)) {
                                                swipeOffsets[favorite.id] = 0
                                            }
                                        } else {
                                            onSelect(favorite); dismiss()
                                        }
                                    }
                                    .offset(x: swipeOffset)
                                    .gesture(DragGesture(minimumDistance: 10)
                                        .onChanged { value in
                                            guard abs(value.translation.width) > abs(value.translation.height) else { return }
                                            guard value.translation.width < 0 else { return }
                                            swipeOffsets[favorite.id] = max(value.translation.width, -110)
                                        }
                                        .onEnded { value in
                                            if value.translation.width < -80 {
                                                withAnimation(.easeOut(duration: 0.2)) {
                                                    swipeOffsets[favorite.id] = -500
                                                }
                                                DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) {
                                                    if let i = favoritesStore.favorites.firstIndex(where: { $0.id == favorite.id }) {
                                                        favoritesStore.remove(at: IndexSet(integer: i))
                                                    }
                                                }
                                            } else {
                                                withAnimation(.spring(response: 0.3, dampingFraction: 0.8)) {
                                                    swipeOffsets[favorite.id] = 0
                                                }
                                            }
                                        }
                                    )
                                }
                                .clipped()
                            }
                        } else {
                            ForEach(completer.results, id: \.self) { completion in
                                searchResultRow(completion)
                            }
                        }
                    }
                    .padding(.horizontal, 16)
                    .padding(.bottom, 16)
                }
            }
            .searchable(text: $searchText, prompt: "Search city or place")
            .onChange(of: searchText) {
                completer.update(query: searchText)
            }
            .navigationTitle("Weather")
            .navigationBarTitleDisplayMode(.large)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button("Done") { dismiss() }
                }
            }
        }
        .preferredColorScheme(.dark)
        .task { await loadWeathers() }
    }

    // MARK: - Cards

    private func locationCard(name: String, isMyLocation: Bool, weather: WeatherResponse?) -> some View {
        let scene = sceneFor(weather)
        return ZStack(alignment: .leading) {
            LinearGradient(
                colors: scene.gradientColors,
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
            .clipShape(RoundedRectangle(cornerRadius: 16))

            VStack(alignment: .leading, spacing: 0) {
                HStack(alignment: .top) {
                    VStack(alignment: .leading, spacing: 3) {
                        Text(name)
                            .font(.title2).bold()
                            .foregroundStyle(.white)
                        if isMyLocation {
                            Label("My Location", systemImage: "location.fill")
                                .font(.subheadline)
                                .foregroundStyle(.white.opacity(0.75))
                        }
                    }
                    Spacer()
                    if let temp = weather?.current.resolvedTemperature {
                        Text("\(Int(temp.rounded()))°")
                            .font(.system(size: 52, weight: .light))
                            .foregroundStyle(.white)
                            .offset(y: -4)
                    }
                }

                Spacer()

                HStack {
                    if let condition = conditionDescription(weather) {
                        Text(condition)
                            .font(.subheadline)
                            .foregroundStyle(.white.opacity(0.85))
                    }
                    Spacer()
                    if let high = weather?.dailyForecast.first?.high,
                       let low = weather?.dailyForecast.first?.low {
                        Text("H:\(Int(high.rounded()))°  L:\(Int(low.rounded()))°")
                            .font(.subheadline)
                            .foregroundStyle(.white.opacity(0.85))
                    }
                }
            }
            .padding(16)
        }
        .frame(height: 120)
    }

    private func searchResultRow(_ completion: MKLocalSearchCompletion) -> some View {
        Button {
            Task {
                let request = MKLocalSearch.Request(completion: completion)
                let response = try? await MKLocalSearch(request: request).start()
                if let item = response?.mapItems.first,
                   let favorite = favoriteLocation(from: item) {
                    favoritesStore.add(favorite)
                    onSelect(favorite)
                }
                dismiss()
            }
        } label: {
            HStack {
                VStack(alignment: .leading, spacing: 3) {
                    Text(completion.title)
                        .font(.body)
                        .foregroundStyle(.white)
                    if !completion.subtitle.isEmpty {
                        Text(completion.subtitle)
                            .font(.subheadline)
                            .foregroundStyle(.white.opacity(0.6))
                    }
                }
                Spacer()
                Image(systemName: "chevron.right")
                    .font(.subheadline)
                    .foregroundStyle(.white.opacity(0.4))
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 14)
            .background(Color.white.opacity(0.08), in: RoundedRectangle(cornerRadius: 12))
        }
    }

    // MARK: - Data loading

    private func loadWeathers() async {
        // Load My Location: cache first, then fetch if missing
        if let coord = currentCoordinate, myLocationWeather == nil {
            if let cached = await weatherService.loadFromCache(lat: coord.latitude, lon: coord.longitude) {
                myLocationWeather = cached
            } else if let fetched = try? await weatherService.fetchWeather(lat: coord.latitude, lon: coord.longitude) {
                await weatherService.saveToCache(fetched, lat: coord.latitude, lon: coord.longitude)
                myLocationWeather = fetched
            }
        }

        // Load each favorite: cache first, then fetch if missing
        await withTaskGroup(of: (UUID, WeatherResponse?).self) { group in
            for favorite in favoritesStore.favorites where favoriteWeathers[favorite.id] == nil {
                group.addTask {
                    if let cached = await weatherService.loadFromCache(lat: favorite.latitude, lon: favorite.longitude) {
                        return (favorite.id, cached)
                    }
                    if let fetched = try? await weatherService.fetchWeather(lat: favorite.latitude, lon: favorite.longitude) {
                        await weatherService.saveToCache(fetched, lat: favorite.latitude, lon: favorite.longitude)
                        return (favorite.id, fetched)
                    }
                    return (favorite.id, nil)
                }
            }
            for await (id, weather) in group {
                if let weather { favoriteWeathers[id] = weather }
            }
        }
    }

    // MARK: - Helpers

    private func sceneFor(_ weather: WeatherResponse?) -> WeatherScene {
        WeatherScene.from(symbolCode:
            weather?.hourlyForecast.first?.symbol ?? weather?.dailyForecast.first?.symbol
        )
    }

    private func conditionDescription(_ weather: WeatherResponse?) -> String? {
        let symbol = weather?.hourlyForecast.first?.symbol ?? weather?.dailyForecast.first?.symbol
        guard let code = symbol.flatMap(Int.init) else { return nil }
        let n = code >= 100 ? code - 100 : code
        switch n {
        case 1:        return "Clear"
        case 2:        return "Partly Cloudy"
        case 3:        return "Mostly Cloudy"
        case 4, 5, 7:  return "Overcast"
        case 6, 9:     return "Fog"
        case 11:       return "Showers"
        case 21:       return "Light Showers"
        case 22:       return "Showers"
        case 23:       return "Heavy Showers"
        case 31:       return "Light Rain"
        case 32:       return "Rain"
        case 33:       return "Heavy Rain"
        case 41:       return "Light Snow"
        case 42:       return "Snow"
        case 43:       return "Heavy Snow"
        case 51:       return "Light Sleet"
        case 52:       return "Sleet"
        case 53:       return "Heavy Sleet"
        case 61, 64:   return "Thunderstorms"
        case 71, 74:   return "Hail"
        default:       return nil
        }
    }

    private func favoriteLocation(from item: MKMapItem) -> FavoriteLocation? {
        guard let coordinate = item.placemark.location?.coordinate else { return nil }
        return FavoriteLocation(
            id: UUID(),
            name: item.name ?? item.placemark.locality ?? "Unknown",
            subtitle: subtitleFor(item),
            latitude: coordinate.latitude,
            longitude: coordinate.longitude
        )
    }

    private func subtitleFor(_ item: MKMapItem) -> String {
        let parts = [item.placemark.locality, item.placemark.countryCode].compactMap { $0 }
        if !parts.isEmpty { return parts.joined(separator: ", ") }
        return item.placemark.country ?? ""
    }
}

// MARK: - Search Completer

@Observable
final class LocationSearchCompleter: NSObject, MKLocalSearchCompleterDelegate {
    var results: [MKLocalSearchCompletion] = []

    private let completer: MKLocalSearchCompleter = {
        let c = MKLocalSearchCompleter()
        c.resultTypes = [.address, .pointOfInterest]
        c.region = MKCoordinateRegion(
            center: CLLocationCoordinate2D(latitude: 64.9, longitude: 25.5),
            span: MKCoordinateSpan(latitudeDelta: 11.0, longitudeDelta: 13.0)
        )
        return c
    }()

    override init() {
        super.init()
        completer.delegate = self
    }

    func update(query: String) {
        if query.count < 2 {
            results = []
        } else {
            completer.queryFragment = query
        }
    }

    func completerDidUpdateResults(_ completer: MKLocalSearchCompleter) {
        results = completer.results.filter { completion in
            let sub = completion.subtitle
            return sub.contains("Finland") || sub.contains("Suomi")
        }
    }

    func completer(_ completer: MKLocalSearchCompleter, didFailWithError error: any Error) {
        results = []
    }
}

// MARK: - Preview

#Preview {
    let store = FavoritesStore()
    store.add(FavoriteLocation(id: UUID(), name: "Tampere", subtitle: "Finland", latitude: 61.4978, longitude: 23.7610))
    store.add(FavoriteLocation(id: UUID(), name: "Turku", subtitle: "Finland", latitude: 60.4518, longitude: 22.2666))

    let mockCurrent = CurrentConditions(
        temperature: -3,
        feelsLike: -8,
        windSpeed: 5,
        windGust: 9,
        windDirection: 220,
        humidity: 82,
        pressure: 1013,
        observedAt: Date()
    )
    let mockDaily = DailyForecast(
        date: "2026-02-26",
        high: 1,
        low: -6,
        symbol: "1",
        windSpeedAvg: 4,
        precipitationMm: 0
    )
    let mockWeather = WeatherResponse(
        station: StationInfo(name: "Helsinki-Vantaa", distanceKm: 2.1),
        current: mockCurrent,
        hourlyForecast: [],
        dailyForecast: [mockDaily]
    )

    return LocationsListView(
        favoritesStore: store,
        weatherService: WeatherService(),
        currentLocationName: "Helsinki",
        currentCoordinate: nil,
        onSelect: { _ in }
    )
}

