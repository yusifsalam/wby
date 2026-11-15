import CoreLocation
import SwiftUI

struct WeatherPageView: View {
    let location: WeatherLocation
    let pageID: WeatherPageID
    let pager: WeatherPagerModel
    let locationService: LocationService
    let weatherService: WeatherService
    let disableAutoLoad: Bool

    @Environment(\.scenePhase) private var scenePhase

    @State private var weather: WeatherResponse?
    @State private var climateNormals: ClimateNormalsResponse?
    @State private var dailyClimateNormals: DailyClimateNormalsResponse?
    @State private var isLoading = false
    @State private var loadedKey: LoadKey?
    @State private var inFlightKey: LoadKey?
    @State private var lastUpdated: Date?
    @State private var showingSavedCopy = false
    @State private var errorMessage: String?
    @State private var selectedDay: DailyForecast?
    @State private var hourlyWindow: [HourlyForecast]?

    private static let staleAfter: TimeInterval = 10 * 60
    private static let forecastStaleAfter: TimeInterval = 6 * 3600

    private struct LoadKey: Hashable {
        let lat: Int
        let lon: Int

        init(_ coordinate: CLLocationCoordinate2D) {
            lat = Int((coordinate.latitude * 100).rounded())
            lon = Int((coordinate.longitude * 100).rounded())
        }
    }

    init(
        location: WeatherLocation,
        pageID: WeatherPageID,
        pager: WeatherPagerModel,
        locationService: LocationService,
        weatherService: WeatherService,
        disableAutoLoad: Bool = false,
        initialWeather: WeatherResponse? = nil
    ) {
        self.location = location
        self.pageID = pageID
        self.pager = pager
        self.locationService = locationService
        self.weatherService = weatherService
        self.disableAutoLoad = disableAutoLoad
        self._weather = State(initialValue: initialWeather)
    }

    // MARK: - Computed coordinate/name/elevation

    private var coordinate: CLLocationCoordinate2D? {
        switch location {
        case .gps: return locationService.coordinate
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

    private var loadKey: LoadKey? {
        coordinate.map(LoadKey.init)
    }

    // MARK: - Scene

    private var currentScene: WeatherScene {
        guard let coordinate else {
            return WeatherSymbols.scene(for: weather)
        }
        let placeTimeZone = weather?.resolvedTimeZone ?? TimeZone(identifier: "Europe/Helsinki")!
        return WeatherSymbols.scene(
            for: weather,
            coordinate: coordinate,
            date: .now,
            timeZone: placeTimeZone,
            elevationMeters: elevationMeters ?? 0
        )
    }

    private func backgroundState(scene: WeatherScene) -> WeatherBackgroundState? {
        guard let weather else { return nil }
        let nextHour = weather.hourlyForecast.first
        return WeatherBackgroundState(
            scene: scene,
            precipitation1h: nextHour?.precipitation1h,
            cloudCover: weather.current.resolvedCloudCover,
            windSpeed: weather.current.resolvedWindSpeed ?? nextHour?.windSpeed,
            windDirection: weather.current.resolvedWindDirection ?? nextHour?.windDirection
        )
    }

    // MARK: - Body

    var body: some View {
        let scene = currentScene
        ScrollView {
            VStack(spacing: 8) {
                if let weather, let coordinate {
                    headerSection(weather, scene: scene)
                    if !weather.hourlyForecast.isEmpty {
                        HourlyForecastCard(
                            hourly: weather.hourlyForecast,
                            timeZone: weather.resolvedTimeZone,
                            coordinate: coordinate,
                            elevationMeters: elevationMeters ?? 0,
                            note: forecastNote(weather)
                        )
                    }
                    CurrentConditionsCard(current: weather.current)
                    dailyForecastSection(weather.dailyForecast, note: forecastNote(weather))
                    HStack(alignment: .top, spacing: 12) {
                        FeelsLikeCard(current: weather.current)
                        UVIndexCard(
                            uvIndex: weather.hourlyForecast.compactMap(\.uvCumulated).first
                                ?? weather.dailyForecast.compactMap(\.uvIndexAvg).first,
                            radiationGlobal: weather.current.resolvedRadiationGlobal
                                ?? dailyResolvedRadiationGlobal(weather.dailyForecast),
                            uvForecast: weather.uvForecast,
                            timeZone: weather.resolvedTimeZone
                        )
                    }
                    WindCard(current: weather.current)
                    HStack(alignment: .top, spacing: 12) {
                        SunriseCard(
                            coordinate: coordinate,
                            referenceDate: weather.current.observedAt,
                            elevationMeters: elevationMeters,
                            timeZone: weather.resolvedTimeZone
                        )
                        PrecipitationCard(forecasts: weather.dailyForecast, timeZone: weather.resolvedTimeZone)
                    }
                    PrecipitationMapCard(
                        coordinate: coordinate,
                        weatherService: weatherService,
                        timeZone: weather.resolvedTimeZone,
                        refreshToken: lastUpdated,
                        disableAutoLoad: disableAutoLoad,
                        onOpenMap: { pager.showingMap = true }
                    )
                    HStack(alignment: .top, spacing: 12) {
                        VisibilityCard(current: weather.current)
                        HumidityCard(current: weather.current)
                    }
                    MoonPhaseCard(
                        coordinate: coordinate,
                        referenceDate: weather.current.observedAt,
                        timeZone: weather.resolvedTimeZone
                    )
                    if let dailyClimateNormals {
                        DailyClimateNormalsCard(
                            normals: dailyClimateNormals,
                            currentTemp: weather.current.resolvedTemperature,
                            currentFeelsLike: weather.current.resolvedFeelsLike,
                            currentWind: weather.current.resolvedWindSpeed,
                            currentHumidity: weather.current.resolvedHumidity,
                            currentSnowDepth: weather.current.snowDepth,
                            todayForecast: weather.dailyForecast.first,
                            timeZone: weather.resolvedTimeZone
                        )
                    } else if let climateNormals {
                        ClimateNormalsCard(
                            normals: climateNormals,
                            currentTemp: weather.current.resolvedTemperature,
                            todayWeatherHigh: weather.dailyForecast.first?.high,
                            todayWeatherLow: weather.dailyForecast.first?.low,
                            timeZone: weather.resolvedTimeZone
                        )
                    }
                    Text(observedCaption(weather))
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .multilineTextAlignment(.center)
                } else if isLoading {
                    ProgressView("Loading weather...")
                        .tint(.primary)
                        .foregroundStyle(.primary)
                        .padding(.top, 100)
                } else if coordinate == nil {
                    LocationUnavailableView(
                        isDenied: locationService.isDenied,
                        didFail: locationService.lastFixFailed
                    )
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
        .onScrollGeometryChange(for: CGFloat.self) { geometry in
            geometry.contentOffset.y
        } action: { _, newValue in
            pager.offsets[pageID] = newValue
        }
        .scrollBounceBehavior(.always)
        .refreshable {
            await Task { await refresh(reload: true) }.value
        }
        .coversWeatherBackground(selectedDay != nil)
        .sheet(item: $selectedDay) { day in
            if let weather, let coordinate {
                NavigationStack {
                    DayDetailView(
                        details: DayDetail.build(
                            days: weather.dailyForecast,
                            hours: hourlyWindow ?? weather.hourlyForecast,
                            timeZone: weather.resolvedTimeZone
                        ),
                        initialDate: day.date,
                        isLoadingHours: hourlyWindow == nil,
                        coordinate: coordinate,
                        elevationMeters: elevationMeters ?? 0
                    )
                }
                .task { await loadHourlyWindow(coord: coordinate) }
            }
        }
        .onChange(of: scenePhase) { _, phase in
            guard phase == .active, !disableAutoLoad, inFlightKey == nil, isStale else { return }
            Task { await refresh(reload: false) }
        }
        .onChange(of: backgroundState(scene: scene), initial: true) { _, state in pager.backgrounds[pageID] = state }
        .task {
            guard case .gps = location, !disableAutoLoad else { return }
            locationService.requestAuthorizationIfNeeded()
        }
        .task(id: loadKey) {
            guard !disableAutoLoad, let coordinate else { return }
            if loadedKey == loadKey, !isStale { return }
            await fetchWeather(coord: coordinate, reload: false)
        }
    }

    // MARK: - Header

    private func headerSection(_ weather: WeatherResponse, scene: WeatherScene) -> some View {
        let primary: Color = scene.prefersLightForeground ? .white : .primary
        let secondary: Color = scene.prefersLightForeground ? .white.opacity(0.78) : .secondary
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
                Text("\(temp.displayText())°")
                    .font(.system(size: 92, weight: .light))
                    .foregroundStyle(primary)
            } else if let forecastTemp = currentHourForecast(weather)?.temperature {
                Text("\(forecastTemp.displayText())°")
                    .font(.system(size: 92, weight: .light))
                    .foregroundStyle(primary)
                Text("Forecast · no recent observation")
                    .font(.subheadline)
                    .foregroundStyle(secondary)
            }
            if let feelsLike = weather.current.resolvedFeelsLike {
                Text("Feels like \(feelsLike.displayText())°")
                    .font(.subheadline)
                    .foregroundStyle(secondary)
            }
            if showingSavedCopy {
                Text("Offline · showing saved data")
                    .font(.subheadline)
                    .foregroundStyle(secondary)
            }
        }
        .padding(.top, 20)
    }

    private func currentHourForecast(_ weather: WeatherResponse) -> HourlyForecast? {
        let now = Date()
        return weather.hourlyForecast.last { $0.time <= now } ?? weather.hourlyForecast.first
    }

    private func observedCaption(_ weather: WeatherResponse) -> String {
        let time = Self.timeWithDate(weather.current.observedAt, timeZone: weather.resolvedTimeZone)
        let km = weather.station.distanceKm
        let distance = km < 10 ? String(format: "%.1f km", km) : "\(Int(km.rounded())) km"
        return "Observed \(time) · \(weather.station.name), \(distance)"
    }

    private func forecastNote(_ weather: WeatherResponse) -> String? {
        guard let fetchedAt = weather.forecastFetchedAt,
              Date().timeIntervalSince(fetchedAt) > Self.forecastStaleAfter
        else { return nil }
        return "Forecast from \(Self.timeWithDate(fetchedAt, timeZone: weather.resolvedTimeZone))"
    }

    private static func timeWithDate(_ date: Date, timeZone: TimeZone) -> String {
        timeFormatter.timeZone = timeZone
        let time = timeFormatter.string(from: date)
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = timeZone
        if calendar.isDate(date, inSameDayAs: Date()) { return time }
        dayFormatter.timeZone = timeZone
        return "\(dayFormatter.string(from: date)), \(time)"
    }

    private static let timeFormatter: DateFormatter = {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.dateFormat = "H.mm"
        return f
    }()

    private static let dayFormatter: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "MMM d"
        return f
    }()

    // MARK: - Daily section

    private func dailyForecastSection(_ forecasts: [DailyForecast], note: String?) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                Label("\(max(0, forecasts.count - 1))-DAY FORECAST", systemImage: "calendar")
                Spacer()
                if let note {
                    Text(note)
                }
            }
            .font(.caption)
            .foregroundStyle(.secondary)
            .padding(.bottom, 8)
            ForEach(forecasts) { day in
                Button {
                    selectedDay = day
                } label: {
                    DailyForecastCard(
                        forecast: day,
                        overallLow: forecasts.compactMap(\.low).min() ?? 0,
                        overallHigh: forecasts.compactMap(\.high).max() ?? 0,
                        timeZone: weather?.resolvedTimeZone ?? TimeZone(identifier: "Europe/Helsinki")!
                    )
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                if day.id != forecasts.last?.id {
                    Divider().overlay(Color.primary.opacity(0.18))
                }
            }
        }
        .weatherCard()
    }

    // MARK: - Loading

    private var isStale: Bool {
        guard let lastUpdated else { return true }
        return Date().timeIntervalSince(lastUpdated) > Self.staleAfter
    }

    private func refresh(reload: Bool) async {
        if case .gps = location {
            let fresh = await locationService.requestFreshLocation()
            guard let coord = fresh ?? locationService.coordinate else { return }
            await fetchWeather(coord: coord, reload: reload)
        } else if let coordinate {
            await fetchWeather(coord: coordinate, reload: reload)
        }
    }

    private func fetchWeather(coord: CLLocationCoordinate2D, reload: Bool) async {
        let key = LoadKey(coord)
        guard inFlightKey != key else { return }
        inFlightKey = key
        isLoading = weather == nil
        defer {
            if inFlightKey == key {
                inFlightKey = nil
                isLoading = false
            }
        }
        do {
            let load = try await weatherService.loadWeather(lat: coord.latitude, lon: coord.longitude, reload: reload)
            guard inFlightKey == key else { return }
            weather = load.weather
            hourlyWindow = nil
            showingSavedCopy = load.isSavedCopy
            lastUpdated = load.isSavedCopy ? nil : Date()
            errorMessage = nil
        } catch WeatherError.httpStatus(404, let message) {
            guard inFlightKey == key else { return }
            weather = nil
            lastUpdated = nil
            loadedKey = nil
            showingSavedCopy = false
            dailyClimateNormals = nil
            climateNormals = nil
            errorMessage = message ?? "No weather data for this location."
            return
        } catch {
            if (error as? WeatherError)?.isCancellation == true { return }
            if inFlightKey == key, weather == nil {
                errorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
            }
            return
        }
        await loadNormals(coord: coord, key: key)
        if inFlightKey == key, !Task.isCancelled {
            loadedKey = key
        }
    }

    private func loadNormals(coord: CLLocationCoordinate2D, key: LoadKey) async {
        do {
            let daily = try await weatherService.fetchDailyClimateNormals(lat: coord.latitude, lon: coord.longitude)
            guard inFlightKey == key else { return }
            dailyClimateNormals = daily
            climateNormals = nil
            return
        } catch WeatherError.httpStatus(404, _) {
            guard inFlightKey == key else { return }
            dailyClimateNormals = nil
        } catch {
            if dailyClimateNormals != nil { return }
        }
        do {
            let monthly = try await weatherService.fetchClimateNormals(lat: coord.latitude, lon: coord.longitude)
            guard inFlightKey == key else { return }
            climateNormals = monthly
        } catch WeatherError.httpStatus(404, _) {
            guard inFlightKey == key else { return }
            climateNormals = nil
        } catch {}
    }

    /// The full ten-day hourly window is fetched lazily the first time a day
    /// sheet opens and kept until the next weather refresh.
    private func loadHourlyWindow(coord: CLLocationCoordinate2D) async {
        guard hourlyWindow == nil else { return }
        do {
            let response = try await weatherService.fetchHourlyForecast(lat: coord.latitude, lon: coord.longitude)
            hourlyWindow = response.hourlyForecast
        } catch {
            hourlyWindow = weather?.hourlyForecast ?? []
        }
    }

    private func dailyResolvedRadiationGlobal(_ daily: [DailyForecast]) -> Double? {
        daily.compactMap(\.radiationGlobalAvg).first.map { max(0, $0) }
    }
}
// MARK: - Preview

enum PreviewData {
    static func makeHourly() -> [HourlyForecast] {
        let now = Date()
        let cal = Calendar.current
        var result: [HourlyForecast] = []
        for i in 0..<12 {
            let precip: Double = (i == 3 || i == 4) ? 0.2 : 0
            let uv: Double = (i > 5 && i < 10) ? 1.5 : 0
            result.append(HourlyForecast(
                time: cal.date(byAdding: .hour, value: i, to: now)!,
                temperature: 4.0 + Double(i) * 0.5,
                windSpeed: 3.2,
                windDirection: 210,
                humidity: 78,
                precipitation1h: precip,
                uvCumulated: uv,
                symbol: "2"
            ))
        }
        return result
    }

    static func makeFullDay() -> [HourlyForecast] {
        var calendar = Calendar.current
        calendar.timeZone = TimeZone(identifier: "Europe/Helsinki")!
        let start = calendar.startOfDay(for: Date())
        return (0..<24).map { i in
            let temp = 8 + 6 * sin(Double(i - 5) / 24 * .pi * 2)
            return HourlyForecast(
                time: calendar.date(byAdding: .hour, value: i, to: start)!,
                temperature: temp,
                feelsLike: temp - 2,
                windSpeed: 3 + Double(i % 5),
                windDirection: Double((i * 30) % 360),
                windGust: 6 + Double(i % 7),
                humidity: 70 + Double(i % 20),
                precipitation1h: (13...16).contains(i) ? Double(16 - i) * 0.6 : 0,
                pop: (12...17).contains(i) ? 60 : 5,
                cloudCover: (11...17).contains(i) ? 85 : 20,
                symbol: (13...16).contains(i) ? "38" : "2"
            )
        }
    }

    static func makeUVForecast() -> [UVPoint] {
        var calendar = Calendar.current
        calendar.timeZone = TimeZone(identifier: "Europe/Helsinki")!
        let start = calendar.startOfDay(for: Date())
        return (0..<48).map { i in
            let hour = i % 24
            let uv = hour >= 6 && hour <= 20 ? max(0, 6.5 * sin(Double(hour - 6) / 14 * .pi)) : 0
            return UVPoint(time: calendar.date(byAdding: .hour, value: i, to: start)!, uv: uv)
        }
    }

    static func makeDaily() -> [DailyForecast] {
        let now = Date()
        var cal = Calendar(identifier: .gregorian)
        let timeZone = TimeZone(identifier: "Europe/Helsinki")!
        cal.timeZone = timeZone
        let symbols = ["2", "1", "3", "21", "1", "2", "3"]
        var result: [DailyForecast] = []
        for i in 0..<7 {
            result.append(DailyForecast(
                date: APIDay.string(cal.date(byAdding: .day, value: i, to: now)!, in: timeZone),
                high: 6.0 + Double(i),
                low: -2.0 + Double(i),
                symbol: symbols[i],
                windSpeedAvg: 4.5,
                precipitationMm: i == 3 ? 2.5 : 0.1
            ))
        }
        return result
    }

    static func makeSample() -> WeatherResponse {
        let current = CurrentConditions(
            temperature: 5.2,
            feelsLike: 2.1,
            windSpeed: 3.8,
            windGust: 7.2,
            windDirection: 215,
            humidity: 82,
            dewPoint: 2.5,
            pressure: 1013.2,
            precipitation1h: 0,
            visibility: 32000,
            cloudCover: 50,
            observedAt: Date()
        )
        return WeatherResponse(
            station: StationInfo(name: "Helsinki Kaisaniemi", distanceKm: 1.2),
            current: current,
            hourlyForecast: makeHourly(),
            dailyForecast: makeDaily(),
            uvForecast: makeUVForecast(),
            timezone: "Europe/Helsinki"
        )
    }
}

#Preview {
    WeatherPageView(
        location: .favorite(.previewHelsinki),
        pageID: .favorite(FavoriteLocation.previewHelsinki.id),
        pager: WeatherPagerModel(),
        locationService: LocationService(),
        weatherService: WeatherService(),
        disableAutoLoad: true,
        initialWeather: PreviewData.makeSample()
    )
}
