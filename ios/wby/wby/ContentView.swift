import CoreLocation
import SwiftUI

struct ContentView: View {
    @State private var locationService = LocationService()
    @State private var weatherService = WeatherService()
    @State private var weather: WeatherResponse?
    @State private var isLoading = false
    @State private var lastUpdated: Date?
    @State private var errorMessage: String?
    private let disableAutoLoad: Bool

    private let fallbackCoordinate = CLLocationCoordinate2D(latitude: 60.1699, longitude: 24.9384)

    init(previewWeather: WeatherResponse? = nil, disableAutoLoad: Bool = false) {
        self._weather = State(initialValue: previewWeather)
        self._lastUpdated = State(initialValue: previewWeather == nil ? nil : Date())
        self.disableAutoLoad = disableAutoLoad
    }

    var body: some View {
        ZStack {
            mainBackground
            ScrollView {
                VStack(spacing: 20) {
                    if let weather {
                        headerSection(weather)
                        if !weather.hourlyForecast.isEmpty {
                            HourlyForecastCard(hourly: weather.hourlyForecast)
                        }
                        CurrentConditionsCard(current: weather.current)
                        dailyForecastSection(weather.dailyForecast)
                        WindCard(
                            current: weather.current,
                            gustSpeed: weather.dailyForecast.first?.windSpeedAvg
                        )
                        if let lastUpdated {
                            Text("Updated \(lastUpdated, style: .relative) ago")
                                .font(.caption)
                                .foregroundStyle(.white.opacity(0.75))
                        }
                    } else if isLoading {
                        ProgressView("Loading weather...")
                            .tint(.white)
                            .foregroundStyle(.white)
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
            .refreshable { await loadWeather() }
        }
        .task {
            guard !disableAutoLoad else { return }
            locationService.requestLocation()
            await loadWeather()
        }
        .onChange(of: locationService.coordinate?.latitude) {
            guard !disableAutoLoad else { return }
            Task { await loadWeather() }
        }
    }

    @ViewBuilder
    private func headerSection(_ weather: WeatherResponse) -> some View {
        VStack(spacing: 4) {
            Text(locationService.placeName ?? weather.station.name)
                .font(.title2)
                .foregroundStyle(.white)
            if let temp = weather.current.temperature {
                Text("\(Int(temp.rounded()))°")
                    .font(.system(size: 72, weight: .thin))
                    .foregroundStyle(.white)
            }
            if let feelsLike = weather.current.feelsLike {
                Text("Feels like \(Int(feelsLike.rounded()))°")
                    .font(.subheadline)
                    .foregroundStyle(.white.opacity(0.78))
            }
        }
        .padding(.top, 20)
    }

    @ViewBuilder
    private func dailyForecastSection(_ forecasts: [DailyForecast]) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            Label("\(forecasts.count-1)-DAY FORECAST", systemImage: "calendar")
                .font(.caption)
                .foregroundStyle(.white.opacity(0.8))
                .padding(.bottom, 8)

            ForEach(forecasts) { day in
                DailyForecastRow(
                    forecast: day,
                    overallLow: forecasts.compactMap(\.low).min() ?? 0,
                    overallHigh: forecasts.compactMap(\.high).max() ?? 0
                )
                if day.id != forecasts.last?.id {
                    Divider()
                        .overlay(Color.white.opacity(0.18))
                }
            }
        }
        .padding()
        .background(forecastCardBackground)
    }

    private var forecastCardBackground: some View {
        RoundedRectangle(cornerRadius: 14, style: .continuous)
            .fill(.clear)
            .background(.ultraThinMaterial.opacity(0.38), in: RoundedRectangle(cornerRadius: 14, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .stroke(Color.white.opacity(0.2), lineWidth: 1)
            )
    }

    private var mainBackground: some View {
        LinearGradient(
            colors: [
                Color(red: 0.38, green: 0.74, blue: 0.99),
                Color(red: 0.23, green: 0.54, blue: 0.94),
                Color(red: 0.11, green: 0.33, blue: 0.73),
            ],
            startPoint: .top,
            endPoint: .bottom
        )
        .ignoresSafeArea()
    }

    private func loadWeather() async {
        guard !disableAutoLoad else { return }
        let coord = locationService.coordinate ?? fallbackCoordinate
        await fetchWeather(lat: coord.latitude, lon: coord.longitude)
    }

    private func fetchWeather(lat: Double, lon: Double) async {
        isLoading = true
        defer { isLoading = false }

        do {
            let response = try await weatherService.fetchWeather(lat: lat, lon: lon)
            weather = response
            lastUpdated = Date()
            errorMessage = nil
            await weatherService.saveToCache(response)
        } catch {
            errorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
            if weather == nil {
                weather = await weatherService.loadFromCache()
            }
        }
    }
}

#Preview {
    ContentView(previewWeather: PreviewWeatherData.response, disableAutoLoad: true)
}

private enum PreviewWeatherData {
    static let response = WeatherResponse(
        station: StationInfo(name: "Kallio", distanceKm: 0.8),
        current: CurrentConditions(
            temperature: -6.0,
            feelsLike: -11.0,
            windSpeed: 3.2,
            windDirection: 250.0,
            humidity: 84.0,
            pressure: 1012.0,
            observedAt: .now
        ),
        hourlyForecast: [
            HourlyForecast(time: .now, temperature: -11.0, symbol: "2"),
            HourlyForecast(time: .now.addingTimeInterval(3600), temperature: -11.0, symbol: "2"),
            HourlyForecast(time: .now.addingTimeInterval(7200), temperature: -11.0, symbol: "3"),
            HourlyForecast(time: .now.addingTimeInterval(10800), temperature: -10.0, symbol: "3"),
            HourlyForecast(time: .now.addingTimeInterval(14400), temperature: -10.0, symbol: "3"),
            HourlyForecast(time: .now.addingTimeInterval(18000), temperature: -10.0, symbol: "3"),
            HourlyForecast(time: .now.addingTimeInterval(21600), temperature: -9.0, symbol: "3"),
            HourlyForecast(time: .now.addingTimeInterval(25200), temperature: -9.0, symbol: "3"),
            HourlyForecast(time: .now.addingTimeInterval(28800), temperature: -9.0, symbol: "3"),
            HourlyForecast(time: .now.addingTimeInterval(32400), temperature: -8.0, symbol: "3"),
            HourlyForecast(time: .now.addingTimeInterval(36000), temperature: -8.0, symbol: "2"),
            HourlyForecast(time: .now.addingTimeInterval(39600), temperature: -8.0, symbol: "2"),
        ],
        dailyForecast: [
            DailyForecast(date: "2026-02-15", high: -5, low: -8, symbol: "3", windSpeedAvg: 3.1, precipitationMm: 0.0),
            DailyForecast(date: "2026-02-16", high: -3, low: -9, symbol: "2", windSpeedAvg: 2.7, precipitationMm: 0.2),
            DailyForecast(date: "2026-02-17", high: -1, low: -6, symbol: "21", windSpeedAvg: 4.2, precipitationMm: 1.1),
            DailyForecast(date: "2026-02-18", high: 1, low: -4, symbol: "1", windSpeedAvg: 2.3, precipitationMm: 0.0),
            DailyForecast(date: "2026-02-19", high: 0, low: -5, symbol: "41", windSpeedAvg: 3.9, precipitationMm: 0.8),
            DailyForecast(date: "2026-02-20", high: -2, low: -7, symbol: "43", windSpeedAvg: 4.4, precipitationMm: 1.6),
            DailyForecast(date: "2026-02-21", high: -4, low: -9, symbol: "3", windSpeedAvg: 3.5, precipitationMm: 0.1),
            DailyForecast(date: "2026-02-22", high: -1, low: -6, symbol: "2", windSpeedAvg: 3.0, precipitationMm: 0.0),
            DailyForecast(date: "2026-02-23", high: 2, low: -3, symbol: "21", windSpeedAvg: 4.1, precipitationMm: 1.4),
            DailyForecast(date: "2026-02-24", high: 1, low: -2, symbol: "3", windSpeedAvg: 2.8, precipitationMm: 0.3),
            DailyForecast(date: "2026-02-25", high: 0, low: -4, symbol: "1", windSpeedAvg: 2.1, precipitationMm: 0.0),
        ]
    )
}
