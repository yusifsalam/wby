import Foundation

actor MapOverlayService {
    private let weatherService: WeatherService

    init(weatherService: WeatherService) {
        self.weatherService = weatherService
    }

    func fetchTemperatureOverlay(bbox: MapBBox, width: Int, height: Int) async throws -> TemperatureOverlayImage {
        try await weatherService.fetchTemperatureOverlay(bbox: bbox, width: width, height: height)
    }

    func fetchTemperatureSamples(at: Date? = nil) async throws -> TemperatureSamplesResponse {
        try await weatherService.fetchTemperatureSamples(at: at)
    }

    func fetchPrecipitationOverlay(
        bbox: MapBBox,
        width: Int,
        height: Int,
        time: Date?
    ) async throws -> PrecipitationOverlayImage {
        try await weatherService.fetchPrecipitationOverlay(
            bbox: bbox,
            width: width,
            height: height,
            time: time
        )
    }
}
