import Foundation

actor WeatherService {
    private let baseURL = URL(string: "http://localhost:8080")!

    func fetchWeather(lat: Double, lon: Double) async throws -> WeatherResponse {
        var components = URLComponents(url: baseURL.appendingPathComponent("v1/weather"), resolvingAgainstBaseURL: false)!
        components.queryItems = [
            URLQueryItem(name: "lat", value: String(format: "%.4f", lat)),
            URLQueryItem(name: "lon", value: String(format: "%.4f", lon)),
        ]

        let (data, response) = try await URLSession.shared.data(from: components.url!)

        guard let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode == 200 else {
            throw WeatherError.serverError
        }

        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try decoder.decode(WeatherResponse.self, from: data)
    }

    func saveToCache(_ response: WeatherResponse) {
        guard let data = try? JSONEncoder().encode(response) else { return }
        try? data.write(to: cacheURL)
    }

    func loadFromCache() -> WeatherResponse? {
        guard let data = try? Data(contentsOf: cacheURL) else { return nil }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try? decoder.decode(WeatherResponse.self, from: data)
    }

    private var cacheURL: URL {
        FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("weather_cache.json")
    }
}

enum WeatherError: Error {
    case serverError
}
