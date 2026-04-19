import Foundation

actor MapOverlayService {
    private let weatherService: WeatherService

    init(weatherService: WeatherService) {
        self.weatherService = weatherService
    }

    func fetchTemperatureOverlay(bbox: MapBBox, width: Int, height: Int) async throws -> TemperatureOverlayImage {
        try await weatherService.fetchTemperatureOverlay(bbox: bbox, width: width, height: height)
    }

    func fetchTemperatureSamples() async throws -> TemperatureSamplesResponse {
        try await weatherService.fetchTemperatureSamples()
    }
}
