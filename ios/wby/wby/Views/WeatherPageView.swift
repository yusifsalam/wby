import CoreLocation
import SwiftUI

struct WeatherPageView: View {
    let location: WeatherLocation
    let locationService: LocationService
    let weatherService: WeatherService
    let disableAutoLoad: Bool
    let pageIndex: Int
    var onSceneChange: ((Int, WeatherScene, Double?) -> Void)?

    @State private var weather: WeatherResponse?
    @State private var isLoading = false
    @State private var lastUpdated: Date?
    @State private var errorMessage: String?

    private let fallbackCoordinate = CLLocationCoordinate2D(latitude: 60.1699, longitude: 24.9384)

    init(location: WeatherLocation, locationService: LocationService, weatherService: WeatherService, disableAutoLoad: Bool = false, pageIndex: Int = 0, onSceneChange: ((Int, WeatherScene, Double?) -> Void)? = nil) {
        self.location = location
        self.locationService = locationService
        self.weatherService = weatherService
        self.disableAutoLoad = disableAutoLoad
        self.pageIndex = pageIndex
        self.onSceneChange = onSceneChange
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
            .toolbarBackground(.hidden, for: .navigationBar)
        .task {
            guard !disableAutoLoad else { return }
            await loadWeather()
        }
        .onAppear {
            onSceneChange?(pageIndex, currentScene, weather?.hourlyForecast.first?.precipitation1h)
        }
        .onChange(of: weather?.current.observedAt) { _, _ in
            onSceneChange?(pageIndex, currentScene, weather?.hourlyForecast.first?.precipitation1h)
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
            Label("\(max(0, forecasts.count - 1))-DAY FORECAST", systemImage: "calendar")
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

    // MARK: - Loading

    private func loadWeather() async {
        let coord = coordinate
        // Show cache immediately
        if let cached = await weatherService.loadFromCache(lat: coord.latitude, lon: coord.longitude) {
            weather = cached
        }
        // Then refresh in background
        await fetchWeather(coord: coord)
    }

    private func fetchWeather(coord: CLLocationCoordinate2D? = nil) async {
        let coord = coord ?? coordinate
        isLoading = weather == nil
        defer { isLoading = false }
        do {
            let response = try await weatherService.fetchWeather(lat: coord.latitude, lon: coord.longitude)
            weather = response
            lastUpdated = Date()
            errorMessage = nil
            await weatherService.saveToCache(response, lat: coord.latitude, lon: coord.longitude)
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
