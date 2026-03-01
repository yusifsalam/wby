import Foundation

struct FavoriteLocation: Codable, Identifiable, Equatable, Hashable {
    let id: UUID
    let name: String       // e.g. "Tampere"
    let subtitle: String   // e.g. "Finland"
    let latitude: Double
    let longitude: Double
}
