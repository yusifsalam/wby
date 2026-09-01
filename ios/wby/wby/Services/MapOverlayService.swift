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

    func fetchPrecipitationForecastGrid(
        bbox: MapBBox,
        width: Int,
        height: Int,
        time: Date?
    ) async throws -> PrecipitationForecastResponse {
        try await weatherService.fetchPrecipitationForecastGrid(
            bbox: bbox,
            width: width,
            height: height,
            time: time
        )
    }

    func fetchPrecipitationObservedGrid(
        bbox: MapBBox,
        width: Int,
        height: Int,
        time: Date?
    ) async throws -> PrecipitationForecastResponse {
        try await weatherService.fetchPrecipitationObservedGrid(
            bbox: bbox,
            width: width,
            height: height,
            time: time
        )
    }

    func fetchPrecipitationNowcastGrid(
        bbox: MapBBox,
        width: Int,
        height: Int,
        time: Date?
    ) async throws -> PrecipitationForecastResponse {
        try await weatherService.fetchPrecipitationNowcastGrid(
            bbox: bbox,
            width: width,
            height: height,
            time: time
        )
    }
}
