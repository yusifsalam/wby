import CoreLocation
import Foundation
import Observation

/// One shared copy of `/v1/weather` per place for the pages, the locations
/// list, and the map pins. Concurrent loads of one place share a request.
final class WeatherStore {
    struct Key: Hashable {
        let lat: Int
        let lon: Int

        init(_ coordinate: CLLocationCoordinate2D) {
            lat = Int((coordinate.latitude * 1000).rounded())
            lon = Int((coordinate.longitude * 1000).rounded())
        }
    }

    struct Entry {
        let weather: WeatherResponse
        let fetchedAt: Date
        let isSavedCopy: Bool

        var isFresh: Bool {
            !isSavedCopy && Date().timeIntervalSince(fetchedAt) < WeatherStore.freshFor
        }
    }

    @Observable
    final class Slot {
        var entry: Entry?
    }

    private struct InFlight {
        let id: Int
        let reload: Bool
        let task: Task<Entry, Error>
    }

    static let freshFor: TimeInterval = 10 * 60

    let service: WeatherService
    private var slots: [Key: Slot] = [:]
    private var inFlight: [Key: InFlight] = [:]
    private var nextID = 0

    init(service: WeatherService = WeatherService(), seed: [(CLLocationCoordinate2D, WeatherResponse)] = []) {
        self.service = service
        for (coordinate, weather) in seed {
            slot(Key(coordinate)).entry = Entry(weather: weather, fetchedAt: Date(), isSavedCopy: false)
        }
    }

    func entry(for key: Key) -> Entry? {
        slot(key).entry
    }

    func entry(for coordinate: CLLocationCoordinate2D) -> Entry? {
        entry(for: Key(coordinate))
    }

    /// A fresh stored entry is returned without a request unless `reload`
    /// is set, which also skips the HTTP cache. A 404 clears the place.
    @discardableResult
    func load(_ coordinate: CLLocationCoordinate2D, reload: Bool = false) async throws -> Entry {
        let key = Key(coordinate)
        if !reload, let entry = slot(key).entry, entry.isFresh { return entry }
        if let running = inFlight[key], !reload || running.reload {
            return try await running.task.value
        }
        nextID += 1
        let id = nextID
        let task = Task<Entry, Error> { [service] in
            defer { if inFlight[key]?.id == id { inFlight[key] = nil } }
            do {
                let load = try await service.loadWeather(
                    lat: coordinate.latitude,
                    lon: coordinate.longitude,
                    reload: reload
                )
                let entry = Entry(weather: load.weather, fetchedAt: Date(), isSavedCopy: load.isSavedCopy)
                if inFlight[key]?.id == id { slot(key).entry = entry }
                return entry
            } catch WeatherError.httpStatus(404, let message) {
                if inFlight[key]?.id == id { slot(key).entry = nil }
                throw WeatherError.httpStatus(404, message)
            }
        }
        inFlight[key] = InFlight(id: id, reload: reload, task: task)
        return try await task.value
    }

    private func slot(_ key: Key) -> Slot {
        if let slot = slots[key] { return slot }
        let slot = Slot()
        slots[key] = slot
        return slot
    }
}
