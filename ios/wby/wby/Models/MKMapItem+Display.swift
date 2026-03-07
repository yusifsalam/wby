import MapKit

extension MKMapItem {
    var areaName: String? {
        nonEmpty(addressRepresentations?.cityName)
            ?? nonEmpty(addressRepresentations?.cityWithContext)
            ?? nonEmpty(address?.shortAddress)
            ?? nonEmpty(addressRepresentations?.regionName)
            ?? nonEmpty(address?.fullAddress)
    }

    var favoriteDisplayName: String? {
        nonEmpty(name)
            ?? nonEmpty(addressRepresentations?.cityName)
            ?? nonEmpty(addressRepresentations?.cityWithContext)
            ?? nonEmpty(address?.shortAddress)
    }

    var favoriteSubtitle: String {
        let parts = [
            nonEmpty(addressRepresentations?.cityName),
            nonEmpty(addressRepresentations?.regionName)
        ].compactMap { $0 }
        if !parts.isEmpty { return parts.joined(separator: ", ") }
        return nonEmpty(address?.shortAddress) ?? nonEmpty(address?.fullAddress) ?? ""
    }
}

private func nonEmpty(_ value: String?) -> String? {
    guard let trimmed = value?.trimmingCharacters(in: .whitespacesAndNewlines), !trimmed.isEmpty else {
        return nil
    }
    return trimmed
}
