import CryptoKit
import Foundation

actor WeatherService {
    private let baseURL: URL?
    private let clientID: String?
    private let clientSecret: String?

    init() {
        baseURL = Self.resolveBaseURL()
        clientID = Self.resolveConfigValue("API_CLIENT_ID")
        clientSecret = Self.resolveConfigValue("API_CLIENT_SECRET")
    }

    func fetchWeather(lat: Double, lon: Double) async throws -> WeatherResponse {
        try await fetchWeather(lat: lat, lon: lon, cachePolicy: .useProtocolCachePolicy)
    }

    /// Weather for a page. `reload` skips the HTTP cache; when the network
    /// fails, the last cached response is returned as a saved copy.
    func loadWeather(lat: Double, lon: Double, reload: Bool) async throws -> WeatherLoad {
        do {
            let weather = try await fetchWeather(
                lat: lat,
                lon: lon,
                cachePolicy: reload ? .reloadIgnoringLocalCacheData : .useProtocolCachePolicy
            )
            return WeatherLoad(weather: weather, isSavedCopy: false)
        } catch let error as WeatherError {
            guard case .network = error, !error.isCancellation,
                  let saved = try? await fetchWeather(lat: lat, lon: lon, cachePolicy: .returnCacheDataDontLoad)
            else { throw error }
            return WeatherLoad(weather: saved, isSavedCopy: true)
        }
    }

    private func fetchWeather(
        lat: Double,
        lon: Double,
        cachePolicy: URLRequest.CachePolicy
    ) async throws -> WeatherResponse {
        try await fetchJSON(
            path: "v1/weather",
            queryItems: [
                URLQueryItem(name: "lat", value: Self.coordinateString(lat)),
                URLQueryItem(name: "lon", value: Self.coordinateString(lon)),
            ],
            dateDecodingStrategy: .iso8601,
            cachePolicy: cachePolicy
        )
    }

    func fetchHourlyForecast(lat: Double, lon: Double) async throws -> HourlyForecastResponse {
        try await fetchJSON(
            path: "v1/weather/hourly",
            queryItems: [
                URLQueryItem(name: "lat", value: Self.coordinateString(lat)),
                URLQueryItem(name: "lon", value: Self.coordinateString(lon)),
            ],
            dateDecodingStrategy: .iso8601
        )
    }

    func fetchTemperatureSamples(at: Date? = nil) async throws -> TemperatureSamplesResponse {
        var queryItems: [URLQueryItem] = []
        if let at {
            queryItems.append(URLQueryItem(name: "at", value: Self.iso8601String(at)))
        }
        return try await fetchJSON(
            path: "v1/map/temperature/samples",
            queryItems: queryItems,
            dateDecodingStrategy: .iso8601
        )
    }

    private static func iso8601String(_ date: Date) -> String {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime]
        return f.string(from: date)
    }

    /// Harmonie precipitation-rate forecast raster (hourly, mm/h), returned as
    /// a grid for client-side texture rendering like the temperature grid.
    func fetchPrecipitationForecastGrid(
        bbox: MapBBox,
        width: Int,
        height: Int,
        time: Date?
    ) async throws -> PrecipitationForecastResponse {
        try await fetchPrecipitationGrid(
            path: "v1/map/precipitation/forecast",
            bbox: bbox, width: width, height: height, time: time
        )
    }

    /// Radar-composite rain-rate raster (5-min frames, mm/h) for past scrubber
    /// frames. Same response shape as the forecast grid so both render through
    /// the texture path.
    func fetchPrecipitationObservedGrid(
        bbox: MapBBox,
        width: Int,
        height: Int,
        time: Date?
    ) async throws -> PrecipitationForecastResponse {
        try await fetchPrecipitationGrid(
            path: "v1/map/precipitation/observed",
            bbox: bbox, width: width, height: height, time: time
        )
    }

    /// Radar-extrapolation nowcast raster (5-min future frames, mm/h),
    /// re-predicted by the server on every radar refresh.
    func fetchPrecipitationNowcastGrid(
        bbox: MapBBox,
        width: Int,
        height: Int,
        time: Date?
    ) async throws -> PrecipitationForecastResponse {
        try await fetchPrecipitationGrid(
            path: "v1/map/precipitation/nowcast",
            bbox: bbox, width: width, height: height, time: time
        )
    }

    private func fetchPrecipitationGrid(
        path: String,
        bbox: MapBBox,
        width: Int,
        height: Int,
        time: Date?
    ) async throws -> PrecipitationForecastResponse {
        var queryItems: [URLQueryItem] = [
            URLQueryItem(name: "bbox", value: bbox.queryValue),
            URLQueryItem(name: "width", value: String(width)),
            URLQueryItem(name: "height", value: String(height)),
        ]
        if let time {
            queryItems.append(URLQueryItem(name: "time", value: Self.iso8601String(time)))
        }
        return try await fetchJSON(
            path: path,
            queryItems: queryItems,
            dateDecodingStrategy: .iso8601
        )
    }

    // MARK: - Climate Normals

    func fetchClimateNormals(lat: Double, lon: Double) async throws -> ClimateNormalsResponse {
        try await fetchJSON(
            path: "v1/climate-normals",
            queryItems: [
                URLQueryItem(name: "lat", value: Self.coordinateString(lat)),
                URLQueryItem(name: "lon", value: Self.coordinateString(lon)),
            ]
        )
    }

    func fetchDailyClimateNormals(lat: Double, lon: Double) async throws -> DailyClimateNormalsResponse {
        try await fetchJSON(
            path: "v1/climate-normals/daily",
            queryItems: [
                URLQueryItem(name: "lat", value: Self.coordinateString(lat)),
                URLQueryItem(name: "lon", value: Self.coordinateString(lon)),
            ],
            dateDecodingStrategy: .iso8601
        )
    }

    // MARK: - Leaderboard

    func fetchLeaderboard(lat: Double, lon: Double, timeframe: String = "now") async throws -> LeaderboardResponse {
        try await fetchJSON(
            path: "v1/leaderboard",
            queryItems: [
                URLQueryItem(name: "lat", value: Self.coordinateString(lat)),
                URLQueryItem(name: "lon", value: Self.coordinateString(lon)),
                URLQueryItem(name: "timeframe", value: timeframe),
            ],
            dateDecodingStrategy: .iso8601
        )
    }

    // MARK: - Private

    /// Builds a signed GET request for `path` with `queryItems`, executes it, and
    /// returns the raw body plus the HTTP response. All shared error translation
    /// (missing base URL, signing, network, HTTP status) happens here.
    private func performRequest(
        path: String,
        queryItems: [URLQueryItem],
        cachePolicy: URLRequest.CachePolicy = .useProtocolCachePolicy
    ) async throws -> (Data, HTTPURLResponse) {
        guard let baseURL else {
            throw WeatherError.missingAPIBaseURL
        }
        var components = URLComponents(
            url: baseURL.appendingPathComponent(path),
            resolvingAgainstBaseURL: false
        )!
        components.queryItems = queryItems
        guard let url = components.url else {
            throw WeatherError.invalidURL
        }
        let query = components.percentEncodedQuery ?? ""

        var request: URLRequest
        do {
            request = try signedRequest(url: url, method: "GET", query: query)
        } catch let error as WeatherError {
            throw error
        } catch {
            throw WeatherError.serverError
        }
        request.cachePolicy = cachePolicy

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await URLSession.shared.data(for: request)
        } catch {
            throw WeatherError.network(error)
        }

        guard let httpResponse = response as? HTTPURLResponse else {
            throw WeatherError.serverError
        }
        guard httpResponse.statusCode == 200 else {
            throw WeatherError.httpStatus(httpResponse.statusCode, Self.extractErrorMessage(data))
        }

        return (data, httpResponse)
    }

    /// Convenience wrapper around `performRequest` that decodes the body as JSON.
    private func fetchJSON<T: Decodable>(
        path: String,
        queryItems: [URLQueryItem],
        dateDecodingStrategy: JSONDecoder.DateDecodingStrategy = .deferredToDate,
        cachePolicy: URLRequest.CachePolicy = .useProtocolCachePolicy
    ) async throws -> T {
        let (data, _) = try await performRequest(path: path, queryItems: queryItems, cachePolicy: cachePolicy)
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = dateDecodingStrategy
        do {
            return try decoder.decode(T.self, from: data)
        } catch {
            throw WeatherError.decoding(error)
        }
    }

    private func signedRequest(url: URL, method: String, query: String) throws -> URLRequest {
        guard let clientID, let clientSecret else {
            throw WeatherError.missingSigningCredentials
        }

        let timestamp = String(Int(Date().timeIntervalSince1970))
        let message = Self.signaturePayload(
            method: method,
            path: url.path,
            query: query,
            timestamp: timestamp
        )
        let signature = Self.hmacSHA256Hex(message: message, secret: clientSecret)

        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue(clientID, forHTTPHeaderField: "X-Client-ID")
        request.setValue(timestamp, forHTTPHeaderField: "X-Timestamp")
        request.setValue(signature, forHTTPHeaderField: "X-Signature")
        return request
    }

    private static func signaturePayload(method: String, path: String, query: String, timestamp: String) -> String {
        method + "\n" + path + "\n" + query + "\n" + timestamp
    }

    private static func hmacSHA256Hex(message: String, secret: String) -> String {
        let key = SymmetricKey(data: Data(secret.utf8))
        let code = HMAC<SHA256>.authenticationCode(for: Data(message.utf8), using: key)
        return code.map { String(format: "%02x", $0) }.joined()
    }

    private static func extractErrorMessage(_ data: Data) -> String? {
        guard !data.isEmpty else {
            return nil
        }
        if let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
           let message = json["error"] as? String,
           !message.isEmpty
        {
            return message
        }
        if let text = String(data: data, encoding: .utf8)?
            .trimmingCharacters(in: .whitespacesAndNewlines),
            !text.isEmpty
        {
            return text
        }
        return nil
    }

    private static func resolveBaseURL() -> URL? {
        guard let configured = resolveConfigValue("API_BASE_URL") else {
            return nil
        }
        return URL(string: configured)
    }

    private static func resolveConfigValue(_ key: String) -> String? {
        guard let value = bundledKeys[key] else {
            return nil
        }
        return normalizedValue(value)
    }

    private static let bundledKeys: [String: String] = {
        guard let url = Bundle.main.url(forResource: "Keys", withExtension: "plist"),
              let data = try? Data(contentsOf: url),
              let plist = try? PropertyListSerialization.propertyList(from: data, format: nil),
              let dict = plist as? [String: Any]
        else {
            return [:]
        }
        var out: [String: String] = [:]
        for (key, value) in dict {
            if let str = value as? String,
               let normalized = normalizedValue(str)
            {
                out[key] = normalized
            }
        }
        return out
    }()

    private static func normalizedValue(_ value: String) -> String? {
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    private static func coordinateString(_ value: Double) -> String {
        String(format: "%.4f", locale: Locale(identifier: "en_US_POSIX"), value)
    }
}

nonisolated struct WeatherLoad {
    let weather: WeatherResponse
    let isSavedCopy: Bool
}

enum WeatherError: Error {
    case invalidURL
    case missingAPIBaseURL
    case missingSigningCredentials
    case network(Error)
    case httpStatus(Int, String?)
    case decoding(Error)
    case serverError

    nonisolated var isCancellation: Bool {
        guard case .network(let error) = self else { return false }
        return error is CancellationError || (error as? URLError)?.code == .cancelled
    }
}

extension WeatherError: LocalizedError {
    var errorDescription: String? {
        switch self {
        case .invalidURL:
            return "Invalid weather API URL."
        case .missingAPIBaseURL:
            return "Missing API_BASE_URL in Keys.plist."
        case .missingSigningCredentials:
            return "Unauthorized! Identify yourself, citizen."
        case .network(let err):
            return "Network error: \(err.localizedDescription)"
        case .httpStatus(let code, let message):
            if let message, !message.isEmpty {
                return "Weather API returned HTTP \(code): \(message)"
            }
            return "Weather API returned HTTP \(code)."
        case .decoding:
            return "Weather response format was invalid."
        case .serverError:
            return "Weather API error."
        }
    }
}
