import CoreLocation
import MapKit
import SwiftUI

struct LocationsListView: View {
    let favoritesStore: FavoritesStore
    let weatherService: WeatherService
    let currentLocationName: String?
    let currentCoordinate: CLLocationCoordinate2D?
    let onSelect: (FavoriteLocation?) -> Void
    let disableAutoLoad: Bool

    @Environment(\.dismiss) private var dismiss
    @State private var myLocationWeather: WeatherResponse?
    @State private var favoriteWeathers: [UUID: WeatherResponse]
    @State private var searchText = ""
    @State private var searchErrorMessage = ""
    @State private var showingSearchError = false
    @State private var completer = LocationSearchCompleter()
    @State private var editMode: EditMode

    init(
        favoritesStore: FavoritesStore,
        weatherService: WeatherService,
        currentLocationName: String?,
        currentCoordinate: CLLocationCoordinate2D?,
        onSelect: @escaping (FavoriteLocation?) -> Void,
        previewEditing: Bool = false,
        disableAutoLoad: Bool = false,
        initialMyLocationWeather: WeatherResponse? = nil,
        initialFavoriteWeathers: [UUID: WeatherResponse] = [:]
    ) {
        self.favoritesStore = favoritesStore
        self.weatherService = weatherService
        self.currentLocationName = currentLocationName
        self.currentCoordinate = currentCoordinate
        self.onSelect = onSelect
        self.disableAutoLoad = disableAutoLoad
        self._myLocationWeather = State(initialValue: initialMyLocationWeather)
        self._favoriteWeathers = State(initialValue: initialFavoriteWeathers)
        self._editMode = State(initialValue: previewEditing ? .active : .inactive)
    }

    private var backgroundColor: Color {
        Color(red: 0.07, green: 0.07, blue: 0.10)
    }

    var body: some View {
        NavigationStack {
            Group {
                if searchText.isEmpty {
                    favoritesList
                } else {
                    searchResultsList
                }
            }
            .scrollContentBackground(.hidden)
            .background { backgroundColor.ignoresSafeArea() }
            .environment(\.editMode, $editMode)
            .searchable(text: $searchText, prompt: "Search city or place")
            .onChange(of: searchText) {
                completer.update(query: searchText)
            }
            .navigationTitle("Weather")
            .navigationBarTitleDisplayMode(.large)
            .toolbar {
                if editMode == .inactive && !favoritesStore.favorites.isEmpty && searchText.isEmpty {
                    ToolbarItem(placement: .topBarLeading) {
                        Button("Edit") {
                            withAnimation { editMode = .active }
                        }
                    }
                }
                ToolbarItem(placement: .topBarTrailing) {
                    if editMode == .active {
                        Button {
                            withAnimation { editMode = .inactive }
                        } label: {
                            Image(systemName: "checkmark.circle")
                                .font(.title2)
                                .symbolRenderingMode(.monochrome)
                        }
                    }
                }
            }
        }
        .preferredColorScheme(.dark)
        .alert("Place Unavailable", isPresented: $showingSearchError) {
            Button("OK", role: .cancel) {}
        } message: {
            Text(searchErrorMessage)
        }
        .task {
            guard !disableAutoLoad else { return }
            await loadWeathers()
        }
    }

    // MARK: - Lists

    private var favoritesList: some View {
        List {
            Section {
                locationCard(
                    name: currentLocationName ?? "My Location",
                    isMyLocation: true,
                    weather: myLocationWeather
                )
                .contentShape(Rectangle())
                .onTapGesture { onSelect(nil); dismiss() }
                .listRowBackground(Color.clear)
                .listRowSeparator(.hidden)
                .listRowInsets(EdgeInsets(top: 6, leading: 16, bottom: 6, trailing: 16))
                .moveDisabled(true)
                .deleteDisabled(true)
            }

            Section {
                ForEach(favoritesStore.favorites) { favorite in
                    locationCard(
                        name: favorite.name,
                        isMyLocation: false,
                        weather: favoriteWeathers[favorite.id]
                    )
                    .contentShape(Rectangle())
                    .onTapGesture {
                        guard editMode == .inactive else { return }
                        onSelect(favorite)
                        dismiss()
                    }
                    .listRowBackground(Color.clear)
                    .listRowSeparator(.hidden)
                    .listRowInsets(EdgeInsets(top: 6, leading: 16, bottom: 6, trailing: 16))
                }
                .onMove { from, to in
                    favoritesStore.move(from: from, to: to)
                }
                .onDelete { offsets in
                    favoritesStore.remove(at: offsets)
                }
            }
        }
        .listStyle(.plain)
        .listSectionSpacing(.compact)
    }

    private var searchResultsList: some View {
        List {
            ForEach(completer.results) { result in
                searchResultRow(result.completion)
                    .listRowBackground(Color.clear)
                    .listRowSeparator(.hidden)
                    .listRowInsets(EdgeInsets(top: 4, leading: 16, bottom: 4, trailing: 16))
            }
        }
        .listStyle(.plain)
    }

    // MARK: - Cards

    private func locationCard(name: String, isMyLocation: Bool, weather: WeatherResponse?) -> some View {
        let scene = WeatherSymbols.scene(for: weather)
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
                        Text("\(temp.displayText())°")
                            .font(.system(size: 52, weight: .light))
                            .foregroundStyle(.white)
                            .offset(y: -4)
                    }
                }

                Spacer()

                HStack {
                    if let condition = WeatherSymbols.conditionDescription(from: weather) {
                        Text(condition)
                            .font(.subheadline)
                            .foregroundStyle(.white.opacity(0.85))
                    }
                    Spacer()
                    if let high = weather?.dailyForecast.first?.high,
                       let low = weather?.dailyForecast.first?.low
                    {
                        Text("H:\(high.displayText())°  L:\(low.displayText())°")
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
            Task { await select(completion) }
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
        .buttonStyle(.plain)
    }

    // MARK: - Data loading

    private func loadWeathers() async {
        // Load My Location weather using URLSession protocol cache semantics.
        if let coord = currentCoordinate, myLocationWeather == nil {
            myLocationWeather = try? await weatherService.fetchWeather(lat: coord.latitude, lon: coord.longitude)
        }

        // Load each favorite weather using URLSession protocol cache semantics.
        await withTaskGroup(of: (UUID, WeatherResponse?).self) { group in
            for favorite in favoritesStore.favorites where favoriteWeathers[favorite.id] == nil {
                group.addTask {
                    let weather = try? await weatherService.fetchWeather(
                        lat: favorite.latitude,
                        lon: favorite.longitude
                    )
                    return (favorite.id, weather)
                }
            }
            for await (id, weather) in group {
                if let weather { favoriteWeathers[id] = weather }
            }
        }
    }

    private func select(_ completion: MKLocalSearchCompletion) async {
        let request = MKLocalSearch.Request(completion: completion)
        guard let item = try? await MKLocalSearch(request: request).start().mapItems.first,
              let favorite = favoriteLocation(from: item)
        else {
            searchErrorMessage = "Couldn't look up that place. Try again."
            showingSearchError = true
            return
        }
        if let region = item.addressRepresentations?.region, region != .finland {
            searchErrorMessage = "Weather is only available for places in Finland."
            showingSearchError = true
            return
        }
        onSelect(favoritesStore.add(favorite))
        dismiss()
    }

    private func favoriteLocation(from item: MKMapItem) -> FavoriteLocation? {
        let coordinate = item.location.coordinate
        guard CLLocationCoordinate2DIsValid(coordinate) else { return nil }
        return FavoriteLocation(
            id: UUID(),
            name: item.favoriteDisplayName ?? "Unknown",
            subtitle: item.favoriteSubtitle,
            latitude: coordinate.latitude,
            longitude: coordinate.longitude
        )
    }
}

// MARK: - Search Completer

struct LocationSearchResult: Identifiable {
    let id: String
    let completion: MKLocalSearchCompletion
}

@Observable
final class LocationSearchCompleter: NSObject, MKLocalSearchCompleterDelegate {
    var results: [LocationSearchResult] = []

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

    private static let finlandNames: [String] = {
        var names = ["Finland", "Suomi"]
        for locale in [Locale.current] + Locale.preferredLanguages.map(Locale.init(identifier:)) {
            if let name = locale.localizedString(forRegionCode: "FI"), !names.contains(name) {
                names.append(name)
            }
        }
        return names
    }()

    func completerDidUpdateResults(_ completer: MKLocalSearchCompleter) {
        var seen = Set<String>()
        results = completer.results.compactMap { completion in
            let sub = completion.subtitle
            guard Self.finlandNames.contains(where: sub.contains) else { return nil }
            let id = "\(completion.title)\n\(sub)"
            guard seen.insert(id).inserted else { return nil }
            return LocationSearchResult(id: id, completion: completion)
        }
    }

    func completer(_ completer: MKLocalSearchCompleter, didFailWithError error: any Error) {
        results = []
    }
}

// MARK: - Preview

#Preview {
    let store = LocationsListPreviewFixture.makeStore()
    let sample = PreviewData.makeSample()
    let firstID = store.favorites.first?.id
    let secondID = store.favorites.dropFirst().first?.id

    LocationsListView(
        favoritesStore: store,
        weatherService: WeatherService(),
        currentLocationName: "Helsinki",
        currentCoordinate: CLLocationCoordinate2D(
            latitude: FavoriteLocation.previewHelsinki.latitude,
            longitude: FavoriteLocation.previewHelsinki.longitude
        ),
        onSelect: { _ in },
        disableAutoLoad: true,
        initialMyLocationWeather: sample,
        initialFavoriteWeathers: {
            var out: [UUID: WeatherResponse] = [:]
            if let firstID { out[firstID] = sample }
            if let secondID { out[secondID] = sample }
            return out
        }()
    )
}

#Preview("Edit Mode") {
    let store = LocationsListPreviewFixture.makeStore()
    let sample = PreviewData.makeSample()
    let firstID = store.favorites.first?.id
    let secondID = store.favorites.dropFirst().first?.id

    LocationsListView(
        favoritesStore: store,
        weatherService: WeatherService(),
        currentLocationName: "Helsinki",
        currentCoordinate: CLLocationCoordinate2D(
            latitude: FavoriteLocation.previewHelsinki.latitude,
            longitude: FavoriteLocation.previewHelsinki.longitude
        ),
        onSelect: { _ in },
        previewEditing: true,
        disableAutoLoad: true,
        initialMyLocationWeather: sample,
        initialFavoriteWeathers: {
            var out: [UUID: WeatherResponse] = [:]
            if let firstID { out[firstID] = sample }
            if let secondID { out[secondID] = sample }
            return out
        }()
    )
}

private enum LocationsListPreviewFixture {
    static func makeStore() -> FavoritesStore {
        FavoritesStore(
            initialFavorites: [
                .previewTampere,
                .previewTurku,
            ],
            persistenceEnabled: false
        )
    }
}
