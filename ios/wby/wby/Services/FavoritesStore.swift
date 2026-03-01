import Foundation
import SwiftUI

@Observable
final class FavoritesStore {
    private(set) var favorites: [FavoriteLocation] = []

    init() { load() }

    func add(_ location: FavoriteLocation) {
        guard !favorites.contains(where: {
            $0.latitude == location.latitude && $0.longitude == location.longitude
        }) else { return }
        favorites.append(location)
        save()
    }

    func remove(at offsets: IndexSet) {
        favorites.remove(atOffsets: offsets)
        save()
    }

    private var storeURL: URL {
        FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("favorites.json")
    }

    private func save() {
        guard let data = try? JSONEncoder().encode(favorites) else { return }
        try? data.write(to: storeURL)
    }

    private func load() {
        guard let data = try? Data(contentsOf: storeURL) else { return }
        favorites = (try? JSONDecoder().decode([FavoriteLocation].self, from: data)) ?? []
    }
}
