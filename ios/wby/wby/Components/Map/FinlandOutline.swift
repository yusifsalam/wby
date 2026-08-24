import MapKit

// Finland coastline, dilated ~2 km seaward, for clipping the temperature
// overlay to land. Unlike the web map — where the basemap water fill paints
// over the overlay edge (web/src/lib/mapOverlay.ts) — the clip edge is exposed
// here. The 2 km buffer makes the clip err toward covering land: shorelines
// and islands stay filled at any zoom, at the cost of a thin tinted strip of
// near-shore water; enclosed waters (buffer holes) are covered too. Islets
// under 1 km^2 are dropped before buffering and isolated buffered blobs under
// 30 km^2 after, so lone skerries don't float as odd discs in open sea.
//
// The rings live in Resources/FinlandOutline.json: flat [lon, lat, ...] arrays
// sorted by area (mainland first), derived from Eurostat GISCO CNTR_RG 1:1M
// 2024 (c) EuroGeographics; buffered, merged, and simplified to ~200 m.
nonisolated enum FinlandOutline {
    struct Ring {
        let points: [MKMapPoint]
        let boundingRect: MKMapRect
    }

    // Projected to MKMapPoint (Web Mercator) once at first use; the overlay
    // renderer converts these to per-tile drawing coordinates on each draw,
    // culling rings that are off-tile or sub-pixel at the current zoom.
    static let rings: [Ring] = load()

    private static func load() -> [Ring] {
        guard
            let url = Bundle.main.url(forResource: "FinlandOutline", withExtension: "json"),
            let data = try? Data(contentsOf: url),
            let raw = try? JSONDecoder().decode([[Double]].self, from: data)
        else {
            assertionFailure("FinlandOutline.json missing or malformed")
            return []
        }
        return raw.compactMap { flat in
            guard flat.count >= 6 else { return nil }
            var points: [MKMapPoint] = []
            points.reserveCapacity(flat.count / 2)
            var minX = Double.greatestFiniteMagnitude
            var minY = Double.greatestFiniteMagnitude
            var maxX = -Double.greatestFiniteMagnitude
            var maxY = -Double.greatestFiniteMagnitude
            for i in stride(from: 0, to: flat.count - 1, by: 2) {
                let point = MKMapPoint(CLLocationCoordinate2D(latitude: flat[i + 1], longitude: flat[i]))
                points.append(point)
                minX = min(minX, point.x)
                minY = min(minY, point.y)
                maxX = max(maxX, point.x)
                maxY = max(maxY, point.y)
            }
            return Ring(
                points: points,
                boundingRect: MKMapRect(x: minX, y: minY, width: maxX - minX, height: maxY - minY)
            )
        }
    }
}
