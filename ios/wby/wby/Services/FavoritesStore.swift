import Foundation
import SwiftUI

@Observable
final class FavoritesStore {
    private let persistenceEnabled: Bool
    private(set) var favorites: [FavoriteLocation] = []

    init(
        initialFavorites: [FavoriteLocation] = [],
        persistenceEnabled: Bool = true
    ) {
        self.persistenceEnabled = persistenceEnabled
        favorites = initialFavorites
        if persistenceEnabled {
            load()
        }
    }

    @discardableResult
    func add(_ location: FavoriteLocation) -> FavoriteLocation {
        if let existing = favorites.first(where: {
            $0.latitude == location.latitude && $0.longitude == location.longitude
        }) {
            return existing
        }
        favorites.append(location)
        save()
        return location
    }

    func remove(at offsets: IndexSet) {
        favorites.remove(atOffsets: offsets)
        save()
    }

    func move(from source: IndexSet, to destination: Int) {
        favorites.move(fromOffsets: source, toOffset: destination)
        save()
    }

    private var storeURL: URL {
        FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("favorites.json")
    }

    private func save() {
        guard persistenceEnabled else { return }
        guard let data = try? JSONEncoder().encode(favorites) else { return }
        try? data.write(to: storeURL, options: .atomic)
    }

    private func load() {
        guard persistenceEnabled else { return }
        guard let data = try? Data(contentsOf: storeURL) else { return }
        do {
            favorites = try JSONDecoder().decode([FavoriteLocation].self, from: data)
        } catch {
            let backupURL = storeURL.deletingLastPathComponent().appendingPathComponent("favorites.unreadable.json")
            try? FileManager.default.removeItem(at: backupURL)
            try? FileManager.default.moveItem(at: storeURL, to: backupURL)
        }
    }
}
