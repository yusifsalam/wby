import CoreLocation
import SwiftUI

struct ContentView: View {
    @State private var locationService = LocationService()
    @State private var weatherService = WeatherService()
    @State private var weather: WeatherResponse?
    @State private var isLoading = false
    @State private var lastUpdated: Date?

    var body: some View {
        ScrollView {
            VStack(spacing: 20) {
                if let weather {
                    headerSection(weather)
                    CurrentConditionsCard(current: weather.current)
                    dailyForecastSection(weather.dailyForecast)
                    if let lastUpdated {
                        Text("Updated \(lastUpdated, style: .relative) ago")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                } else if isLoading {
                    ProgressView("Loading weather...")
                        .padding(.top, 100)
                } else {
                    ContentUnavailableView(
                        "No Weather Data",
                        systemImage: "cloud.slash",
                        description: Text("Pull down to refresh")
                    )
                }
            }
            .padding()
        }
        .refreshable { await loadWeather() }
        .task {
            locationService.requestLocation()
            await loadWeather()
        }
        .onChange(of: locationService.coordinate?.latitude) {
            Task { await loadWeather() }
        }
    }

    @ViewBuilder
    private func headerSection(_ weather: WeatherResponse) -> some View {
        VStack(spacing: 4) {
            Text(locationService.placeName ?? weather.station.name)
                .font(.title2)
            if let temp = weather.current.temperature {
                Text("\(Int(temp.rounded()))°")
                    .font(.system(size: 72, weight: .thin))
            }
            if let feelsLike = weather.current.feelsLike {
                Text("Feels like \(Int(feelsLike.rounded()))°")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.top, 20)
    }

    @ViewBuilder
    private func dailyForecastSection(_ forecasts: [DailyForecast]) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            Label("10-DAY FORECAST", systemImage: "calendar")
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
                    Divider()
                }
            }
        }
        .padding()
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 12))
    }

    private func loadWeather() async {
        guard let coord = locationService.coordinate else {
            if weather == nil {
                weather = await weatherService.loadFromCache()
            }
            return
        }
        await fetchWeather(lat: coord.latitude, lon: coord.longitude)
    }

    private func fetchWeather(lat: Double, lon: Double) async {
        isLoading = true
        defer { isLoading = false }

        do {
            let response = try await weatherService.fetchWeather(lat: lat, lon: lon)
            weather = response
            lastUpdated = Date()
            await weatherService.saveToCache(response)
        } catch {
            if weather == nil {
                weather = await weatherService.loadFromCache()
            }
        }
    }
}

#Preview {
    ContentView()
}
