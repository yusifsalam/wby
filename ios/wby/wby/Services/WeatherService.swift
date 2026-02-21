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
        guard let baseURL else {
            throw WeatherError.missingAPIBaseURL
        }
        var components = URLComponents(url: baseURL.appendingPathComponent("v1/weather"), resolvingAgainstBaseURL: false)!
        components.queryItems = [
            URLQueryItem(name: "lat", value: Self.coordinateString(lat)),
            URLQueryItem(name: "lon", value: Self.coordinateString(lon)),
        ]
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

    private static func resolveBaseURL() -> URL? {
        guard let configured = resolveConfigValue("API_BASE_URL") else {
            return nil
        }
        return URL(string: configured)
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

    private static func resolveConfigValue(_ key: String) -> String? {
        guard let value = bundledKeys[key] else {
            return nil
        }
        return normalizedValue(value)
    }

    private static var bundledKeys: [String: String] = {
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
