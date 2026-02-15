import Foundation

actor WeatherService {
    private let baseURL: URL

    init() {
        self.baseURL = Self.resolveBaseURL()
    }

    func fetchWeather(lat: Double, lon: Double) async throws -> WeatherResponse {
        var components = URLComponents(url: baseURL.appendingPathComponent("v1/weather"), resolvingAgainstBaseURL: false)!
        components.queryItems = [
            URLQueryItem(name: "lat", value: String(format: "%.4f", lat)),
            URLQueryItem(name: "lon", value: String(format: "%.4f", lon)),
        ]
        guard let url = components.url else {
            throw WeatherError.invalidURL
        }

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await URLSession.shared.data(from: url)
        } catch {
            throw WeatherError.network(error)
        }

        guard let httpResponse = response as? HTTPURLResponse else {
            throw WeatherError.serverError
        }
        guard httpResponse.statusCode == 200 else {
            throw WeatherError.httpStatus(httpResponse.statusCode)
        }

        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        do {
            return try decoder.decode(WeatherResponse.self, from: data)
        } catch {
            throw WeatherError.decoding(error)
        }
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

    private static func resolveBaseURL() -> URL {
        if let configured = Bundle.main.object(forInfoDictionaryKey: "API_BASE_URL") as? String,
           !configured.isEmpty,
           let url = URL(string: configured) {
            return url
        }
        #if targetEnvironment(simulator)
        return URL(string: "http://localhost:8080")!
        #else
        return URL(string: "http://127.0.0.1:8080")!
        #endif
    }
}

enum WeatherError: Error {
    case invalidURL
    case network(Error)
    case httpStatus(Int)
    case decoding(Error)
    case serverError
}

extension WeatherError: LocalizedError {
    var errorDescription: String? {
        switch self {
        case .invalidURL:
            return "Invalid weather API URL."
        case .network(let err):
            return "Network error: \(err.localizedDescription)"
        case .httpStatus(let code):
            return "Weather API returned HTTP \(code)."
        case .decoding:
            return "Weather response format was invalid."
        case .serverError:
            return "Weather API error."
        }
    }
}
