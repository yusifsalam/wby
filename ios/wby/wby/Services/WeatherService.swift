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
        try await fetchJSON(
            path: "v1/weather",
            queryItems: [
                URLQueryItem(name: "lat", value: Self.coordinateString(lat)),
                URLQueryItem(name: "lon", value: Self.coordinateString(lon)),
            ],
            dateDecodingStrategy: .iso8601
        )
    }

    func fetchTemperatureSamples() async throws -> TemperatureSamplesResponse {
        return try await fetchJSON(
            path: "v1/map/temperature/samples",
            queryItems: [],
            dateDecodingStrategy: .iso8601
        )
    }

    func fetchTemperatureOverlay(bbox: MapBBox, width: Int, height: Int) async throws -> TemperatureOverlayImage {
        let bboxValue = [
            Self.coordinateString(bbox.minLon),
            Self.coordinateString(bbox.minLat),
            Self.coordinateString(bbox.maxLon),
            Self.coordinateString(bbox.maxLat),
        ].joined(separator: ",")
        let (data, httpResponse) = try await performRequest(
            path: "v1/map/temperature",
            queryItems: [
                URLQueryItem(name: "bbox", value: bboxValue),
                URLQueryItem(name: "width", value: String(width)),
                URLQueryItem(name: "height", value: String(height)),
            ]
        )

        let dataTime = httpResponse.value(forHTTPHeaderField: "X-Data-Time").flatMap(Self.parseDate)
        let minTemp = httpResponse.value(forHTTPHeaderField: "X-Temp-Min").flatMap(Double.init)
        let maxTemp = httpResponse.value(forHTTPHeaderField: "X-Temp-Max").flatMap(Double.init)

        return TemperatureOverlayImage(
            imageData: data,
            bbox: bbox,
            dataTime: dataTime,
            minTemp: minTemp,
            maxTemp: maxTemp
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
        queryItems: [URLQueryItem]
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

        let request: URLRequest
        do {
            request = try signedRequest(url: url, method: "GET", query: query)
        } catch let error as WeatherError {
            throw error
        } catch {
            throw WeatherError.serverError
        }

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
        dateDecodingStrategy: JSONDecoder.DateDecodingStrategy = .deferredToDate
    ) async throws -> T {
        let (data, _) = try await performRequest(path: path, queryItems: queryItems)
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

    private static func parseDate(_ value: String) -> Date? {
        let iso = ISO8601DateFormatter()
        return iso.date(from: value)
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

enum WeatherError: Error {
    case invalidURL
    case missingAPIBaseURL
    case missingSigningCredentials
    case network(Error)
    case httpStatus(Int, String?)
    case decoding(Error)
    case serverError
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
