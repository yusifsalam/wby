import CoreLocation
import MapKit
import SwiftUI
import UniformTypeIdentifiers

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
    @State private var isEditing: Bool

    init(
        favoritesStore: FavoritesStore,
        weatherService: WeatherService,
        currentLocationName: String?,
        currentCoordinate: CLLocationCoordinate2D?,
        onSelect: @escaping (FavoriteLocation?) -> Void,
        previewEditing: Bool = false
    ) {
        self.favoritesStore = favoritesStore
        self.weatherService = weatherService
        self.currentLocationName = currentLocationName
        self.currentCoordinate = currentCoordinate
        self.onSelect = onSelect
        self._isEditing = State(initialValue: previewEditing)
    }

    @State private var draggingItem: FavoriteLocation?

    var body: some View {
        NavigationStack {
            ZStack {
                Color(red: 0.07, green: 0.07, blue: 0.10)
                    .ignoresSafeArea()
                    .onDrop(
                        of: [UTType.text],
                        delegate: FavoriteCancelDropDelegate(draggingItem: $draggingItem)
                    )

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
                                let swipeOffset = isEditing ? 0 : (swipeOffsets[favorite.id] ?? 0)

                                HStack(spacing: 10) {
                                    if isEditing {
                                        Button {
                                            withAnimation {
                                                if let i = favoritesStore.favorites.firstIndex(of: favorite) {
                                                    favoritesStore.remove(at: IndexSet(integer: i))
                                                }
                                            }
                                        } label: {
                                            Image(systemName: "minus.circle.fill")
                                                .font(.title2)
                                                .foregroundStyle(.red)
                                        }
                                        .transition(.move(edge: .leading).combined(with: .opacity))
                                    }

                                    ZStack(alignment: .trailing) {
                                        RoundedRectangle(cornerRadius: 16)
                                            .fill(.red)
                                            .overlay(alignment: .trailing) {
                                                Image(systemName: "trash")
                                                    .foregroundStyle(.white)
                                                    .font(.title2)
                                                    .padding(.trailing, 24)
                                            }
                                            .opacity(isEditing ? 0 : min(1, -swipeOffset / 60))

                                        locationCard(
                                            name: favorite.name,
                                            isMyLocation: false,
                                            weather: favoriteWeathers[favorite.id]
                                        )
                                        .onTapGesture {
                                            guard !isEditing else { return }
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
                                                guard !isEditing else { return }
                                                guard abs(value.translation.width) > abs(value.translation.height) else { return }
                                                guard value.translation.width < 0 else { return }
                                                swipeOffsets[favorite.id] = max(value.translation.width, -110)
                                            }
                                            .onEnded { value in
                                                guard !isEditing else { return }
                                                if value.translation.width < -80 {
                                                    withAnimation(.easeOut(duration: 0.2)) {
                                                        swipeOffsets[favorite.id] = -500
                                                    }
                                                    Task { @MainActor in
                                                        try? await Task.sleep(for: .seconds(0.2))
                                                        if let i = favoritesStore.favorites.firstIndex(where: { $0.id == favorite.id }) {
                                                            favoritesStore.remove(at: IndexSet(integer: i))
                                                        }
                                                    }
                                                } else {
                                                    withAnimation(.spring(response: 0.3, dampingFraction: 0.8)) {
                                                        swipeOffsets[favorite.id] = 0
                                                    }
                                                }
                                            })
                                    }
                                    .clipped()

                                    if isEditing {
                                        Image(systemName: "line.3.horizontal")
                                            .foregroundStyle(.white.opacity(0.4))
                                            .font(.title3)
                                            .transition(.move(edge: .trailing).combined(with: .opacity))
                                    }
                                }
                                .onDrag {
                                    draggingItem = favorite
                                    return NSItemProvider(object: favorite.id.uuidString as NSString)
                                }
                                .onDrop(
                                    of: [UTType.text],
                                    delegate: FavoriteReorderDelegate(
                                        item: favorite,
                                        favoritesStore: favoritesStore,
                                        draggingItem: $draggingItem
                                    )
                                )
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
                if !isEditing && !favoritesStore.favorites.isEmpty && searchText.isEmpty {
                    ToolbarItem(placement: .topBarLeading) {
                        Button("Edit") {
                            withAnimation { isEditing = true }
                        }
                    }
                }
                ToolbarItem(placement: .topBarTrailing) {
                    if isEditing {
                        Button {
                            withAnimation { isEditing = false }
                        } label: {
                            Image(systemName: "checkmark.circle")
                                .font(.title2)
                                .symbolRenderingMode(.monochrome)
                                .foregroundStyle(.white.opacity(0.7))
                        }
                    }
                }
            }
        }
        .preferredColorScheme(.dark)
        .task { await loadWeathers() }
        .onChange(of: isEditing) {
            if isEditing { swipeOffsets.removeAll() }
        }
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
                       let low = weather?.dailyForecast.first?.low
                    {
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
                   let favorite = favoriteLocation(from: item)
                {
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
            weather?.hourlyForecast.first?.symbol ?? weather?.dailyForecast.first?.symbol)
    }

    private func conditionDescription(_ weather: WeatherResponse?) -> String? {
        let symbol = weather?.hourlyForecast.first?.symbol ?? weather?.dailyForecast.first?.symbol
        guard let code = symbol.flatMap(Int.init) else { return nil }
        let n = code >= 100 ? code - 100 : code
        switch n {
        case 1: return "Clear"
        case 2: return "Partly Cloudy"
        case 3: return "Mostly Cloudy"
        case 4, 5, 7: return "Overcast"
        case 6, 9: return "Fog"
        case 11: return "Showers"
        case 21: return "Light Showers"
        case 22: return "Showers"
        case 23: return "Heavy Showers"
        case 31: return "Light Rain"
        case 32: return "Rain"
        case 33: return "Heavy Rain"
        case 41: return "Light Snow"
        case 42: return "Snow"
        case 43: return "Heavy Snow"
        case 51: return "Light Sleet"
        case 52: return "Sleet"
        case 53: return "Heavy Sleet"
        case 61, 64: return "Thunderstorms"
        case 71, 74: return "Hail"
        default: return nil
        }
    }

    private func favoriteLocation(from item: MKMapItem) -> FavoriteLocation? {
        let coordinate = item.location.coordinate
        guard CLLocationCoordinate2DIsValid(coordinate) else { return nil }
        return FavoriteLocation(
            id: UUID(),
            name: nonEmpty(item.name)
                ?? nonEmpty(item.addressRepresentations?.cityName)
                ?? nonEmpty(item.addressRepresentations?.cityWithContext)
                ?? nonEmpty(item.address?.shortAddress)
                ?? "Unknown",
            subtitle: subtitleFor(item),
            latitude: coordinate.latitude,
            longitude: coordinate.longitude
        )
    }

    private func subtitleFor(_ item: MKMapItem) -> String {
        let parts = [
            nonEmpty(item.addressRepresentations?.cityName),
            nonEmpty(item.addressRepresentations?.regionName)
        ].compactMap { $0 }
        if !parts.isEmpty { return parts.joined(separator: ", ") }
        return nonEmpty(item.address?.shortAddress) ?? nonEmpty(item.address?.fullAddress) ?? ""
    }

    private func nonEmpty(_ value: String?) -> String? {
        guard let trimmed = value?.trimmingCharacters(in: .whitespacesAndNewlines), !trimmed.isEmpty else {
            return nil
        }
        return trimmed
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

// MARK: - Reorder Drop Delegate

struct FavoriteReorderDelegate: DropDelegate {
    let item: FavoriteLocation
    let favoritesStore: FavoritesStore
    @Binding var draggingItem: FavoriteLocation?

    func performDrop(info: DropInfo) -> Bool {
        draggingItem = nil
        return true
    }

    func dropEntered(info: DropInfo) {
        guard let dragging = draggingItem,
              dragging != item,
              let fromIndex = favoritesStore.favorites.firstIndex(of: dragging),
              let toIndex = favoritesStore.favorites.firstIndex(of: item)
        else { return }
        withAnimation {
            favoritesStore.move(
                from: IndexSet(integer: fromIndex),
                to: toIndex > fromIndex ? toIndex + 1 : toIndex
            )
        }
    }

    func dropExited(info: DropInfo) {
        // Clear if drag left all targets (e.g. cancelled or dropped outside)
        if !favoritesStore.favorites.contains(where: { $0 == draggingItem }) {
            draggingItem = nil
        }
    }

    func dropUpdated(info: DropInfo) -> DropProposal? {
        DropProposal(operation: .move)
    }
}

/// Placed on the ScrollView background to catch drops outside any card and clear drag state.
struct FavoriteCancelDropDelegate: DropDelegate {
    @Binding var draggingItem: FavoriteLocation?

    func performDrop(info: DropInfo) -> Bool {
        draggingItem = nil
        return true
    }

    func dropUpdated(info: DropInfo) -> DropProposal? {
        DropProposal(operation: .cancel)
    }
}

// MARK: - Preview

#Preview {
    let store = FavoritesStore()
    store.add(FavoriteLocation(id: UUID(), name: "Tampere", subtitle: "Finland", latitude: 61.4978, longitude: 23.7610))
    store.add(FavoriteLocation(id: UUID(), name: "Turku", subtitle: "Finland", latitude: 60.4518, longitude: 22.2666))

    return LocationsListView(
        favoritesStore: store,
        weatherService: WeatherService(),
        currentLocationName: "Helsinki",
        currentCoordinate: nil,
        onSelect: { _ in }
    )
}

#Preview("Edit Mode") {
    let store = FavoritesStore()
    store.add(FavoriteLocation(id: UUID(), name: "Tampere", subtitle: "Finland", latitude: 61.4978, longitude: 23.7610))
    store.add(FavoriteLocation(id: UUID(), name: "Turku", subtitle: "Finland", latitude: 60.4518, longitude: 22.2666))

    return LocationsListView(
        favoritesStore: store,
        weatherService: WeatherService(),
        currentLocationName: "Helsinki",
        currentCoordinate: nil,
        onSelect: { _ in },
        previewEditing: true
    )
}
