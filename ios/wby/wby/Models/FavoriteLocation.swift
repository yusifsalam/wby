import Foundation

struct FavoriteLocation: Codable, Identifiable, Equatable, Hashable {
    let id: UUID
    let name: String       // e.g. "Tampere"
    let subtitle: String   // e.g. "Finland"
    let latitude: Double
    let longitude: Double
}

#if DEBUG
extension FavoriteLocation {
    static let previewHelsinki = FavoriteLocation(
        id: UUID(uuidString: "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEE1")!,
        name: "Helsinki",
        subtitle: "Finland",
        latitude: 60.1699,
        longitude: 24.9384
    )

    static let previewTampere = FavoriteLocation(
        id: UUID(uuidString: "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEE2")!,
        name: "Tampere",
        subtitle: "Finland",
        latitude: 61.4978,
        longitude: 23.7610
    )

    static let previewTurku = FavoriteLocation(
        id: UUID(uuidString: "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEE3")!,
        name: "Turku",
        subtitle: "Finland",
        latitude: 60.4518,
        longitude: 22.2666
    )

    static let weatherMapPreviewFavorites = [
        previewHelsinki,
        previewTampere,
        previewTurku,
    ]
}
#endif
