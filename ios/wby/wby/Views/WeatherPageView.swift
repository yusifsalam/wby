import CoreLocation
import SwiftUI

struct WeatherPageView: View {
    let location: WeatherLocation
    let locationService: LocationService
    let weatherService: WeatherService
    let disableAutoLoad: Bool
    let onBackgroundUpdate: (WeatherScene, Double?) -> Void

    @State private var weather: WeatherResponse?
    @State private var climateNormals: ClimateNormalsResponse?
    @State private var isLoading = false
    @State private var lastUpdated: Date?
    @State private var errorMessage: String?

    private let fallbackCoordinate = CLLocationCoordinate2D(latitude: 60.1699, longitude: 24.9384)

    init(
        location: WeatherLocation,
        locationService: LocationService,
        weatherService: WeatherService,
        disableAutoLoad: Bool = false,
        initialWeather: WeatherResponse? = nil,
        onBackgroundUpdate: @escaping (WeatherScene, Double?) -> Void = { _, _ in }
    ) {
        self.location = location
        self.locationService = locationService
        self.weatherService = weatherService
        self.disableAutoLoad = disableAutoLoad
        self._weather = State(initialValue: initialWeather)
        self.onBackgroundUpdate = onBackgroundUpdate
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
        WeatherSymbols.scene(
            for: weather,
            coordinate: coordinate,
            date: .now,
            elevationMeters: elevationMeters ?? 0
        )
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
                    if let climateNormals {
                        ClimateNormalsCard(
                            normals: climateNormals,
                            currentTemp: weather.current.resolvedTemperature
                        )
                    }
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
        .refreshable {
            if case .gps = location {
                let freshCoord = await locationService.requestFreshLocation()
                await fetchWeather(coord: freshCoord)
            } else {
                await fetchWeather()
            }
        }
        .onAppear { publishBackground() }
        .onChange(of: currentScene) { _, _ in publishBackground() }
        .onChange(of: weather?.hourlyForecast.first?.precipitation1h) { _, _ in publishBackground() }
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
        await fetchWeather(coord: coordinate)
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
        } catch {
            if weather == nil {
                errorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
            }
        }
        do {
            climateNormals = try await weatherService.fetchClimateNormals(lat: coord.latitude, lon: coord.longitude)
        } catch {
            climateNormals = nil
        }
    }

    private func dailyResolvedRadiationGlobal(_ daily: [DailyForecast]) -> Double? {
        daily.compactMap(\.radiationGlobalAvg).first.map { max(0, $0) }
    }

    private func publishBackground() {
        onBackgroundUpdate(currentScene, weather?.hourlyForecast.first?.precipitation1h)
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

    static func makeDaily() -> [DailyForecast] {
        let now = Date()
        let cal = Calendar.current
        let fmt = DateFormatter()
        fmt.dateFormat = "yyyy-MM-dd"
        let symbols = ["2", "1", "3", "21", "1", "2", "3"]
        var result: [DailyForecast] = []
        for i in 0..<7 {
            result.append(DailyForecast(
                date: fmt.string(from: cal.date(byAdding: .day, value: i, to: now)!),
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
            dailyForecast: makeDaily()
        )
    }
}

#Preview {
    WeatherPageView(
        location: .favorite(FavoriteLocation(
            id: UUID(),
            name: "Helsinki",
            subtitle: "Finland",
            latitude: 60.1699,
            longitude: 24.9384
        )),
        locationService: LocationService(),
        weatherService: WeatherService(),
        disableAutoLoad: true,
        initialWeather: PreviewData.makeSample()
    )
}
