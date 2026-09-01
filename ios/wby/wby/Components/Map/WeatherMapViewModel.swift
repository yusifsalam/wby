import CoreLocation
import MapKit
import UIKit

struct OverlayMeta: Equatable {
    let dataTime: Date?
    let minTemp: Double?
    let maxTemp: Double?
}

enum TemperatureText {
    static func rounded(_ value: Double?) -> Int? {
        value.map { Int($0.rounded()) }
    }

    static func value(_ value: Int?) -> String {
        value.map { "\($0)°" } ?? "--°"
    }

    static func range(low: Int?, high: Int?) -> String {
        "L \(value(low))  H \(value(high))"
    }
}

struct FavoritePinWeather: Equatable {
    let current: Int?
    let low: Int?
    let high: Int?

    static func from(response: WeatherResponse) -> FavoritePinWeather {
        let today = response.dailyForecast.first
        return FavoritePinWeather(
            current: TemperatureText.rounded(response.current.resolvedTemperature ?? response.hourlyForecast.first?.temperature),
            low: TemperatureText.rounded(today?.low),
            high: TemperatureText.rounded(today?.high)
        )
    }
}

private struct CachedFavoritePinWeather {
    let weather: FavoritePinWeather
    let fetchedAt: Date
}

enum PreviewLoadState {
    case loading
    case loaded
    case failed
}

struct PreviewWeather: Equatable {
    let current: Int?
    let conditionText: String?
    let symbolCode: String?
    let low: Int?
    let high: Int?

    static func from(response: WeatherResponse) -> PreviewWeather {
        let today = response.dailyForecast.first
        return PreviewWeather(
            current: TemperatureText.rounded(response.current.resolvedTemperature ?? response.hourlyForecast.first?.temperature),
            conditionText: WeatherSymbols.conditionDescription(from: response),
            symbolCode: WeatherSymbols.primarySymbol(from: response),
            low: TemperatureText.rounded(today?.low),
            high: TemperatureText.rounded(today?.high)
        )
    }
}

struct OverlaySeed {
    let image: UIImage
    let bbox: MapBBox
}

/// One precipitation timeline frame as a ready-to-apply overlay image. The
/// 1h (WMS) layer decodes a server PNG; the 12h layer renders the GRIB grid
/// client-side via the metal renderer.
private struct PrecipFrame {
    let image: UIImage
    let dataTime: Date?
    let bbox: MapBBox
}

@MainActor
@Observable
final class WeatherMapViewModel {
    var meta: OverlayMeta?
    var overlayMode: OverlayMode
    var selectedLayer: MapLayerKind
    var selectedTime: Date = Date()
    var isPlaying: Bool = false

    var scrubberPastSteps: Int { selectedLayer.scrubberPastSteps }
    var scrubberFutureSteps: Int { selectedLayer.scrubberFutureSteps }
    var scrubberStepSeconds: TimeInterval { selectedLayer.scrubberStepSeconds }
    var isScrubberVisible: Bool {
        selectedLayer.usesPrecipitationFrames || (selectedLayer == .temperature && overlayMode == .metal)
    }

    private let playbackInterval: TimeInterval = 0.6

    private let favoriteWeatherTTL: TimeInterval = 10 * 60
    private let favoritePinsMaxLatitudeDelta: CLLocationDegrees = 5.8
    private let overlayRefreshInterval: TimeInterval = 3 * 60
    private let overlaySize: Int = 512
    private let overlayService: MapOverlayService
    private let weatherService: WeatherService
    private let networkEnabled: Bool
    private let initialOverlaySeed: OverlaySeed?
    private let initialSamples: TemperatureSamplesResponse?
    private let metalRenderer: TemperatureMetalRenderer?
    // Internal plumbing — never read by views, so kept out of observation
    // tracking. This also keeps the nonisolated deinit able to touch the task
    // handles (tracked properties become main-actor-isolated accessors).
    @ObservationIgnored private weak var mapView: MKMapView?
    @ObservationIgnored private var overlayTask: Task<Void, Never>?
    @ObservationIgnored private var overlayLastFetchedAt: Date?
    @ObservationIgnored private var samplesTask: Task<Void, Never>?
    @ObservationIgnored private var samplesLastFetchedAt: Date?
    @ObservationIgnored private var cachedSamples: TemperatureSamplesResponse?
    @ObservationIgnored private var samplesByHour: [Date: TemperatureSamplesResponse] = [:]
    @ObservationIgnored private var inflightSampleHours: Set<Date> = []
    @ObservationIgnored private var sparseRetriedHours: Set<Date> = []
    // Matches server's ForecastBackfillThreshold (weather/service.go).
    // Below this, the server triggers backfill and shortens its cache window;
    // a retry above this just hits the server's 5 min cache and gets the same
    // payload back.
    private let sparseSampleThreshold: Int = 30
    private let sparseRetryDelay: TimeInterval = 15
    @ObservationIgnored private var prefetchTask: Task<Void, Never>?
    @ObservationIgnored private var playbackTask: Task<Void, Never>?
    @ObservationIgnored private var cachedOverlayImage: TemperatureOverlayImage?
    @ObservationIgnored private var cachedMetalOverlaySeed: OverlaySeed?
    @ObservationIgnored private var precipImagesByHour: [Date: PrecipFrame] = [:]
    // The 12h precip grid is fetched over a slightly padded bbox so the rendered
    // Finland extent stays strictly inside the grid (no transparent edge strip).
    private let precipGridBBox = MapBBox(minLon: 18.8, minLat: 58.8, maxLon: 32.2, maxLat: 71.7)
    @ObservationIgnored private var inflightPrecipHours: Set<Date> = []
    @ObservationIgnored private var precipPrefetchTask: Task<Void, Never>?
    @ObservationIgnored private var favoriteWeatherCache: [UUID: CachedFavoritePinWeather] = [:]
    @ObservationIgnored private var favoriteWeatherTasks: [UUID: Task<Void, Never>] = [:]
    @ObservationIgnored private var previewAnnotation: PreviewPinAnnotation?
    @ObservationIgnored private var previewWeatherTask: Task<Void, Never>?
    @ObservationIgnored private var previewGeocodeTask: Task<Void, Never>?
    private let previewHaptic = UIImpactFeedbackGenerator(style: .light)
    @ObservationIgnored private var favoriteLocations: [FavoriteLocation] = []
    @ObservationIgnored private var preferredCenter: CLLocationCoordinate2D?
    @ObservationIgnored private var didCenterOnPreferredLocation = false

    init(
        overlayService: MapOverlayService,
        weatherService: WeatherService,
        networkEnabled: Bool = true,
        initialMeta: OverlayMeta? = nil,
        initialFavoriteWeather: [UUID: FavoritePinWeather] = [:],
        initialOverlaySeed: OverlaySeed? = nil,
        initialSamples: TemperatureSamplesResponse? = nil,
        overlayMode: OverlayMode = OverlayMode.load(),
        selectedLayer: MapLayerKind = MapLayerKind.load()
    ) {
        self.overlayService = overlayService
        self.weatherService = weatherService
        self.networkEnabled = networkEnabled
        self.initialOverlaySeed = initialOverlaySeed
        self.initialSamples = initialSamples
        self.metalRenderer = TemperatureMetalRenderer()
        self.meta = initialMeta
        self.selectedLayer = selectedLayer
        self.selectedTime = Self.snap(Date(), toSeconds: selectedLayer.scrubberStepSeconds)
        let resolvedMode: OverlayMode
        if overlayMode == .metal, metalRenderer == nil {
            resolvedMode = .png
        } else {
            resolvedMode = overlayMode
        }
        self.overlayMode = resolvedMode
        if resolvedMode != overlayMode {
            resolvedMode.save()
        }
        self.favoriteWeatherCache = initialFavoriteWeather.mapValues {
            CachedFavoritePinWeather(weather: $0, fetchedAt: Date())
        }
        if let initialSamples {
            self.cachedSamples = initialSamples
        }
    }

    deinit {
        overlayTask?.cancel()
        samplesTask?.cancel()
        prefetchTask?.cancel()
        precipPrefetchTask?.cancel()
        playbackTask?.cancel()
        favoriteWeatherTasks.values.forEach { $0.cancel() }
        previewWeatherTask?.cancel()
        previewGeocodeTask?.cancel()
    }

    func bind(mapView: MKMapView) {
        guard self.mapView !== mapView else { return }
        self.mapView = mapView
        didCenterOnPreferredLocation = false
        applyInitialRegion(on: mapView)
        if overlayMode == .png, selectedLayer == .temperature, let initialOverlaySeed {
            applyOverlay(
                image: initialOverlaySeed.image,
                bbox: initialOverlaySeed.bbox,
                clipsToFinland: true,
                on: mapView
            )
        }
        updateFavoriteAnnotations(on: mapView, favorites: favoriteLocations)
        applyVisibilityForCurrentState()
        kickOffRefreshForCurrentState()
    }

    func setOverlayMode(_ mode: OverlayMode) {
        guard mode != overlayMode else { return }
        if mode == .metal, metalRenderer == nil {
            return
        }
        if mode != .metal {
            // Tear down timeline-only state so playback and prefetch don't keep
            // running after the scrubber is hidden.
            stopPlayback()
            prefetchTask?.cancel()
            prefetchTask = nil
            inflightSampleHours.removeAll()
        }
        overlayMode = mode
        mode.save()
        applyVisibilityForCurrentState()
        if selectedLayer == .temperature {
            if let cachedSamples, mode == .metal {
                meta = OverlayMeta(
                    dataTime: cachedSamples.dataTime,
                    minTemp: cachedSamples.minTemp,
                    maxTemp: cachedSamples.maxTemp
                )
            } else if let cachedOverlayImage, mode == .png {
                meta = OverlayMeta(
                    dataTime: cachedOverlayImage.dataTime,
                    minTemp: cachedOverlayImage.minTemp,
                    maxTemp: cachedOverlayImage.maxTemp
                )
            }
        }
        kickOffRefreshForCurrentState()
    }

    func setSelectedLayer(_ layer: MapLayerKind) {
        guard layer != selectedLayer else { return }
        // Tear down timeline state — both layers reuse the scrubber but with
        // different ranges and frame caches.
        stopPlayback()
        prefetchTask?.cancel()
        prefetchTask = nil
        precipPrefetchTask?.cancel()
        precipPrefetchTask = nil
        inflightSampleHours.removeAll()
        inflightPrecipHours.removeAll()
        // The two precipitation layers use different step granularities (5min vs
        // hourly), so their cached frames aren't interchangeable.
        precipImagesByHour.removeAll()
        selectedLayer = layer
        layer.save()
        // Snap selectedTime back into the new layer's range.
        selectedTime = snapToStep(Date())
        applyVisibilityForCurrentState()
        kickOffRefreshForCurrentState()
    }

    func setFavoriteLocations(_ favorites: [FavoriteLocation]) {
        favoriteLocations = favorites
        guard let mapView else { return }
        updateFavoriteAnnotations(on: mapView, favorites: favorites)
    }

    func setPreferredCenter(_ coordinate: CLLocationCoordinate2D?) {
        preferredCenter = coordinate
        guard let mapView else { return }
        guard let coordinate, !didCenterOnPreferredLocation else { return }
        mapView.setRegion(
            MKCoordinateRegion(
                center: coordinate,
                span: MKCoordinateSpan(latitudeDelta: 1.2, longitudeDelta: 1.2)
            ),
            animated: true
        )
        didCenterOnPreferredLocation = true
    }

    func handleRegionDidChange(on mapView: MKMapView) {
        applyFavoritePinVisibility(on: mapView)
        refreshVisibleFavoriteWeather(on: mapView)
        // Refresh only the throttled "latest" overlay on region changes.
        // Don't touch precipitation here: its overlay covers all of Finland
        // and prefetchTimeline would cancel the in-flight fetches every pan.
        switch selectedLayer {
        case .temperature:
            switch overlayMode {
            case .png:
                scheduleOverlayRefresh(on: mapView)
            case .metal:
                scheduleSamplesRefresh()
            }
        case .precipitation, .precipitation12h:
            break
        }
    }

    func handleLongPress(_ recognizer: UILongPressGestureRecognizer) {
        guard recognizer.state == .began,
              let mapView = recognizer.view as? MKMapView
        else { return }

        let point = recognizer.location(in: mapView)
        let coordinate = mapView.convert(point, toCoordinateFrom: mapView)

        cancelPreviewTasks()
        removePreviewAnnotation(from: mapView)

        let annotation = PreviewPinAnnotation(coordinate: coordinate)
        annotation.placeName = formatFallbackPlaceName(coordinate)
        previewAnnotation = annotation
        mapView.addAnnotation(annotation)

        previewHaptic.impactOccurred()
        guard networkEnabled else {
            annotation.loadState = .failed
            refreshPreviewView(for: annotation, on: mapView)
            return
        }
        startPreviewWeatherFetch(for: annotation, on: mapView)
        startPreviewGeocode(for: annotation, on: mapView)
    }

    func dismissPreview(on mapView: MKMapView) {
        cancelPreviewTasks()
        removePreviewAnnotation(from: mapView)
    }

    func updateFavoriteAnnotations(on mapView: MKMapView, favorites: [FavoriteLocation]) {
        let validIDs = Set(favorites.map(\.id))
        let staleTaskIDs = favoriteWeatherTasks.keys.filter { !validIDs.contains($0) }
        for id in staleTaskIDs {
            favoriteWeatherTasks[id]?.cancel()
            favoriteWeatherTasks[id] = nil
            favoriteWeatherCache[id] = nil
        }

        let existing = mapView.favoritePinAnnotations
        if !existing.isEmpty {
            mapView.removeAnnotations(existing)
        }
        let newAnnotations = favorites.map {
            FavoritePinAnnotation(
                id: $0.id,
                title: $0.name,
                coordinate: CLLocationCoordinate2D(latitude: $0.latitude, longitude: $0.longitude),
                weather: favoriteWeatherCache[$0.id]?.weather
            )
        }
        mapView.addAnnotations(newAnnotations)
        applyFavoritePinVisibility(on: mapView)
        refreshVisibleFavoriteWeather(on: mapView)
    }

    func scheduleOverlayRefresh(on mapView: MKMapView) {
        guard networkEnabled else { return }
        if let lastFetched = overlayLastFetchedAt,
           Date().timeIntervalSince(lastFetched) < overlayRefreshInterval
        {
            return
        }

        overlayTask?.cancel()
        overlayTask = Task { [weak mapView] in
            guard !Task.isCancelled else { return }
            do {
                let overlayImage = try await overlayService.fetchTemperatureOverlay(
                    bbox: .finland,
                    width: overlaySize,
                    height: overlaySize
                )
                guard !Task.isCancelled, let mapView, let image = UIImage(data: overlayImage.imageData) else { return }
                overlayLastFetchedAt = Date()
                cachedOverlayImage = overlayImage
                if overlayMode == .png {
                    applyOverlay(
                        image: image,
                        bbox: overlayImage.bbox,
                        clipsToFinland: true,
                        on: mapView
                    )
                    meta = OverlayMeta(
                        dataTime: overlayImage.dataTime,
                        minTemp: overlayImage.minTemp,
                        maxTemp: overlayImage.maxTemp
                    )
                }
            } catch {
                // Keep previous successful overlay visible on fetch errors.
            }
        }
    }

    func scheduleSamplesRefresh() {
        guard networkEnabled else { return }
        if let last = samplesLastFetchedAt,
           Date().timeIntervalSince(last) < overlayRefreshInterval
        {
            return
        }

        samplesTask?.cancel()
        samplesTask = Task { [weak self] in
            guard let self else { return }
            guard !Task.isCancelled else { return }
            do {
                let resp = try await overlayService.fetchTemperatureSamples()
                guard !Task.isCancelled else { return }
                samplesLastFetchedAt = Date()
                cachedSamples = resp
                samplesByHour[snapToStep(Date())] = resp
                if isAtLiveNow(selectedTime) {
                    applyMetalFrame(from: resp)
                }
            } catch {
                // Keep previous samples on fetch errors.
            }
        }
    }

    func setSelectedTime(_ date: Date) {
        let snapped = snapToStep(date)
        selectedTime = snapped
        applySelectedTime()
    }

    func togglePlayback() {
        if isPlaying {
            stopPlayback()
        } else {
            startPlayback()
        }
    }

    func prefetchTimeline() {
        guard networkEnabled else { return }
        let buckets = scrubberStepBuckets()
        let now = snapToStep(Date())
        // Fan out from "now" so the visible frame loads first.
        let ordered = buckets.sorted {
            abs($0.timeIntervalSince(now)) < abs($1.timeIntervalSince(now))
        }
        let validBuckets = Set(buckets)
        switch selectedLayer {
        case .temperature:
            guard overlayMode == .metal else { return }
            prefetchTask?.cancel()
            samplesByHour = samplesByHour.filter { validBuckets.contains($0.key) }
            prefetchTask = Task { [weak self] in
                for bucket in ordered {
                    if Task.isCancelled { return }
                    guard let self else { return }
                    if samplesByHour[bucket] != nil { continue }
                    if inflightSampleHours.contains(bucket) { continue }
                    await fetchSamples(for: bucket)
                }
            }
        case .precipitation, .precipitation12h:
            precipPrefetchTask?.cancel()
            precipImagesByHour = precipImagesByHour.filter { validBuckets.contains($0.key) }
            precipPrefetchTask = Task { [weak self] in
                for bucket in ordered {
                    if Task.isCancelled { return }
                    guard let self else { return }
                    if precipImagesByHour[bucket] != nil { continue }
                    if inflightPrecipHours.contains(bucket) { continue }
                    await fetchPrecipFrame(for: bucket)
                }
            }
        }
    }

    private func applySelectedTime() {
        let snapped = snapToStep(selectedTime)
        switch selectedLayer {
        case .temperature:
            guard overlayMode == .metal else { return }
            if let cached = samplesByHour[snapped] {
                applyMetalFrame(from: cached)
                return
            }
            guard networkEnabled else { return }
            Task { [weak self] in
                guard let self else { return }
                await fetchSamples(for: snapped)
            }
        case .precipitation, .precipitation12h:
            invalidatePrecipFramesIfRampChanged()
            if let cached = precipImagesByHour[snapped] {
                applyPrecipFrame(cached)
                return
            }
            guard networkEnabled else { return }
            Task { [weak self] in
                guard let self else { return }
                await fetchPrecipFrame(for: snapped)
            }
        }
    }

    private func fetchSamples(for hour: Date) async {
        if inflightSampleHours.contains(hour) { return }
        inflightSampleHours.insert(hour)
        defer { inflightSampleHours.remove(hour) }
        do {
            let at: Date? = isAtLiveNow(hour) ? nil : hour
            let resp = try await overlayService.fetchTemperatureSamples(at: at)
            guard !Task.isCancelled else { return }
            samplesByHour[hour] = resp
            if snapToStep(selectedTime) == hour {
                applyMetalFrame(from: resp)
            }
            // The dense GRIB grid is never sparse; only the station-sample
            // fallback (grid == nil) can trip the backfill retry.
            if resp.grid == nil && resp.samples.count < sparseSampleThreshold && !sparseRetriedHours.contains(hour) {
                scheduleSparseRetry(for: hour)
            }
        } catch {
            // Keep previous frames on fetch errors; user can retry by scrubbing.
        }
    }

    private func scheduleSparseRetry(for hour: Date) {
        sparseRetriedHours.insert(hour)
        let delayNs = UInt64(sparseRetryDelay * 1_000_000_000)
        Task { [weak self] in
            try? await Task.sleep(nanoseconds: delayNs)
            if Task.isCancelled { return }
            guard let self else { return }
            // Evict all sparse cache entries so future scrubs re-fetch from
            // the server (which by now should have completed its forecast
            // grid backfill).
            self.samplesByHour = self.samplesByHour.filter {
                $0.value.grid != nil || $0.value.samples.count >= self.sparseSampleThreshold
            }
            await self.fetchSamples(for: hour)
        }
    }

    // Rendered precip frames bake the ramp in; when the Settings style changes,
    // drop them so scrubbing re-renders instead of mixing styles.
    @ObservationIgnored private var renderedPrecipRamp = PrecipRampStyle.load()

    private func invalidatePrecipFramesIfRampChanged() {
        let ramp = PrecipRampStyle.load()
        guard ramp != renderedPrecipRamp else { return }
        renderedPrecipRamp = ramp
        precipPrefetchTask?.cancel()
        precipImagesByHour.removeAll()
    }

    // The grid field for the active precipitation ramp, selecting the Metal
    // fragment (smooth gradient vs stepped radar classes).
    private var precipGridField: GridField {
        PrecipRampStyle.load() == .stepped ? .precipitationStepped : .precipitation
    }

    private func fetchPrecipFrame(for hour: Date) async {
        invalidatePrecipFramesIfRampChanged()
        if inflightPrecipHours.contains(hour) { return }
        inflightPrecipHours.insert(hour)
        defer { inflightPrecipHours.remove(hour) }
        do {
            let target: Date? = isAtLiveNow(hour) ? nil : hour
            let frame: PrecipFrame?
            switch selectedLayer {
            case .precipitation12h:
                frame = try await loadPrecipForecastFrame(time: target)
            default:
                frame = try await loadNearTermPrecipFrame(time: target)
            }
            guard !Task.isCancelled, let frame else { return }
            precipImagesByHour[hour] = frame
            let snappedSelected = snapToStep(selectedTime)
            if snappedSelected == hour, selectedLayer.usesPrecipitationFrames {
                applyPrecipFrame(frame)
            } else if selectedLayer.usesPrecipitationFrames,
                      let mapView,
                      mapView.overlays.compactMap({ $0 as? TemperatureImageOverlay }).isEmpty {
                // No exact frame for the selected step yet (e.g. it 502'd).
                // Show this frame as a placeholder so the map isn't blank.
                applyPrecipFrame(frame)
            }
        } catch {
            // Keep previous frames; user can retry by scrubbing.
        }
    }

    // Near-term scrubber frame: past/now frames come from the radar
    // observation grid, future frames from the extrapolation nowcast grid;
    // both render client-side like the 12h forecast. A radar miss (frame gap,
    // older server) falls back to the WMS tiles so the scrubber never goes
    // blank.
    private func loadNearTermPrecipFrame(time: Date?) async throws -> PrecipFrame? {
        let isPastOrNow = time == nil || time! <= Date()
        do {
            let frame = try await (isPastOrNow
                ? loadPrecipObservedFrame(time: time)
                : loadPrecipNowcastFrame(time: time))
            if let frame { return frame }
        } catch {
            // Fall through to the WMS tiles.
        }
        return try await loadPrecipTileFrame(time: time)
    }

    // Radar observation grid (5-min frames): fetch and render via texture
    // bilinear, the same path as the 12h forecast grid.
    private func loadPrecipObservedFrame(time: Date?) async throws -> PrecipFrame? {
        renderPrecipGridFrame(
            try await overlayService.fetchPrecipitationObservedGrid(
                bbox: precipGridBBox,
                width: overlaySize,
                height: overlaySize,
                time: time
            ))
    }

    // Radar-extrapolation nowcast grid (5-min future frames).
    private func loadPrecipNowcastFrame(time: Date?) async throws -> PrecipFrame? {
        renderPrecipGridFrame(
            try await overlayService.fetchPrecipitationNowcastGrid(
                bbox: precipGridBBox,
                width: overlaySize,
                height: overlaySize,
                time: time
            ))
    }

    // WMS tile fallback: decode the server PNG into an overlay image.
    private func loadPrecipTileFrame(time: Date?) async throws -> PrecipFrame? {
        let response = try await overlayService.fetchPrecipitationOverlay(
            bbox: .finland,
            width: overlaySize,
            height: overlaySize,
            time: time
        )
        guard let image = UIImage(data: response.imageData) else { return nil }
        return PrecipFrame(image: image, dataTime: response.dataTime, bbox: response.bbox)
    }

    // 12h (Harmonie) forecast: fetch the GRIB raster and render it client-side
    // via texture bilinear, the same path as the temperature grid overlay.
    private func loadPrecipForecastFrame(time: Date?) async throws -> PrecipFrame? {
        renderPrecipGridFrame(
            try await overlayService.fetchPrecipitationForecastGrid(
                bbox: precipGridBBox,
                width: overlaySize,
                height: overlaySize,
                time: time
            ))
    }

    // Renders a precipitation grid response through the Metal texture path with
    // the active ramp.
    private func renderPrecipGridFrame(_ response: PrecipitationForecastResponse) -> PrecipFrame? {
        guard let renderer = metalRenderer,
              let grid = response.grid,
              renderer.setGrid(grid, field: precipGridField),
              let image = renderer.renderImage(
                  bounds: MercatorBounds.finland,
                  width: overlaySize,
                  height: overlaySize
              )
        else { return nil }
        return PrecipFrame(image: image, dataTime: response.dataTime, bbox: .finland)
    }

    private func applyPrecipFrame(_ frame: PrecipFrame) {
        guard selectedLayer.usesPrecipitationFrames else { return }
        guard let mapView else { return }
        meta = OverlayMeta(dataTime: frame.dataTime, minTemp: nil, maxTemp: nil)
        applyOverlay(image: frame.image, bbox: frame.bbox, on: mapView)
    }

    private func applyMetalFrame(from response: TemperatureSamplesResponse) {
        cachedSamples = response
        updateMetalOverlayCache(from: response)
        if overlayMode == .metal {
            meta = OverlayMeta(
                dataTime: response.dataTime,
                minTemp: response.minTemp,
                maxTemp: response.maxTemp
            )
            if let seed = cachedMetalOverlaySeed, let mapView {
                applyOverlay(image: seed.image, bbox: seed.bbox, clipsToFinland: true, on: mapView)
            }
        }
    }

    private func startPlayback() {
        guard !isPlaying else { return }
        isPlaying = true
        prefetchTimeline()
        playbackTask?.cancel()
        let intervalNs = UInt64(playbackInterval * 1_000_000_000)
        playbackTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(nanoseconds: intervalNs)
                if Task.isCancelled { return }
                guard let self else { return }
                if !self.isPlaying { return }
                self.advancePlaybackOneStep()
            }
        }
    }

    private func stopPlayback() {
        isPlaying = false
        playbackTask?.cancel()
        playbackTask = nil
    }

    private func advancePlaybackOneStep() {
        let buckets = scrubberStepBuckets()
        guard !buckets.isEmpty else { return }
        let snapped = snapToStep(selectedTime)
        let nextIndex: Int
        if let current = buckets.firstIndex(of: snapped) {
            nextIndex = (current + 1) % buckets.count
        } else {
            nextIndex = 0
        }
        setSelectedTime(buckets[nextIndex])
    }

    private func scrubberStepBuckets() -> [Date] {
        let step = scrubberStepSeconds
        let now = snapToStep(Date())
        let past = scrubberPastSteps
        let future = scrubberFutureSteps
        var buckets: [Date] = []
        buckets.reserveCapacity(past + future + 1)
        for offset in -past...future {
            buckets.append(now.addingTimeInterval(TimeInterval(offset) * step))
        }
        return buckets
    }

    private func isAtLiveNow(_ date: Date) -> Bool {
        snapToStep(date) == snapToStep(Date())
    }

    /// Snap a date to the active layer's scrubber step (1h for temperature,
    /// 5min for precipitation). Used as the cache key for both samples and
    /// precipitation frames.
    func snapToStep(_ date: Date) -> Date {
        Self.snap(date, toSeconds: scrubberStepSeconds)
    }

    static func snap(_ date: Date, toSeconds interval: TimeInterval) -> Date {
        // Floor (not nearest) so "live now" always lands on the most recent
        // completed step. For precipitation that means we ask for the most
        // recently published observation rather than a 5-min mark in the
        // future, which would route to the (flakier) forecast layer.
        let floored = (date.timeIntervalSince1970 / interval).rounded(.down) * interval
        return Date(timeIntervalSince1970: floored)
    }

    private func kickOffRefreshForCurrentState() {
        guard let mapView else { return }
        switch selectedLayer {
        case .temperature:
            switch overlayMode {
            case .png:
                scheduleOverlayRefresh(on: mapView)
            case .metal:
                scheduleSamplesRefresh()
            }
        case .precipitation, .precipitation12h:
            // Drive the same prefetch that scrubbing uses, ordered closest-to-now,
            // so the visible frame loads first and the rest of the timeline streams in.
            let now = snapToStep(Date())
            if let cached = precipImagesByHour[now] {
                applyPrecipFrame(cached)
            }
            prefetchTimeline()
        }
    }

    private func applyVisibilityForCurrentState() {
        guard let mapView else { return }
        // Always start by clearing any previous overlay so we never stack a
        // temperature tile under a precipitation tile or vice versa.
        let existing = mapView.overlays.compactMap { $0 as? TemperatureImageOverlay }
        if !existing.isEmpty {
            mapView.removeOverlays(existing)
        }
        switch selectedLayer {
        case .temperature:
            switch overlayMode {
            case .png:
                if let cachedOverlayImage,
                   let image = UIImage(data: cachedOverlayImage.imageData) {
                    applyOverlay(image: image, bbox: cachedOverlayImage.bbox, clipsToFinland: true, on: mapView)
                }
            case .metal:
                if let seed = cachedMetalOverlaySeed {
                    applyOverlay(image: seed.image, bbox: seed.bbox, clipsToFinland: true, on: mapView)
                } else if let cachedSamples {
                    updateMetalOverlayCache(from: cachedSamples)
                    if let seed = cachedMetalOverlaySeed {
                        applyOverlay(image: seed.image, bbox: seed.bbox, clipsToFinland: true, on: mapView)
                    }
                }
            }
        case .precipitation, .precipitation12h:
            let hour = snapToStep(selectedTime)
            if let cached = precipImagesByHour[hour] {
                applyPrecipFrame(cached)
            }
        }
    }

    private func updateMetalOverlayCache(from response: TemperatureSamplesResponse) {
        guard let renderer = metalRenderer else { return }
        // Forecast frames carry the dense GRIB raster (texture bilinear); station
        // fallbacks (now/past) carry irregular samples (point IDW).
        if let grid = response.grid {
            renderer.setGrid(grid)
        } else {
            renderer.setSamples(response.samples)
        }
        guard let image = renderer.renderImage(
            bounds: MercatorBounds.finland,
            width: overlaySize,
            height: overlaySize
        ) else { return }
        cachedMetalOverlaySeed = OverlaySeed(image: image, bbox: .finland)
    }

    private func startPreviewWeatherFetch(
        for annotation: PreviewPinAnnotation,
        on mapView: MKMapView
    ) {
        previewWeatherTask = Task { [weak self, weak mapView] in
            guard let self else { return }
            do {
                let response = try await self.weatherService.fetchWeather(
                    lat: annotation.coordinate.latitude,
                    lon: annotation.coordinate.longitude
                )
                guard let mapView = activePreviewMapView(for: annotation, mapView: mapView) else { return }

                annotation.weather = PreviewWeather.from(response: response)
                annotation.loadState = .loaded
                refreshPreviewView(for: annotation, on: mapView)
            } catch WeatherError.httpStatus(404, _) {
                guard let mapView = activePreviewMapView(for: annotation, mapView: mapView) else { return }
                self.dismissPreview(on: mapView)
            } catch {
                guard let mapView = activePreviewMapView(for: annotation, mapView: mapView) else { return }
                annotation.loadState = .failed
                refreshPreviewView(for: annotation, on: mapView)
            }
        }
    }

    private func startPreviewGeocode(
        for annotation: PreviewPinAnnotation,
        on mapView: MKMapView
    ) {
        let location = CLLocation(
            latitude: annotation.coordinate.latitude,
            longitude: annotation.coordinate.longitude
        )
        previewGeocodeTask = Task { [weak self, weak mapView] in
            guard let self else { return }
            let mapItems: [MKMapItem]
            if let request = MKReverseGeocodingRequest(location: location) {
                mapItems = (try? await request.mapItems) ?? []
            } else {
                mapItems = []
            }
            guard let mapView = activePreviewMapView(for: annotation, mapView: mapView) else { return }

            let resolved = mapItems.first?.areaName
            annotation.placeName = resolved ?? formatFallbackPlaceName(annotation.coordinate)
            refreshPreviewView(for: annotation, on: mapView)
        }
    }

    private func refreshVisibleFavoriteWeather(on mapView: MKMapView) {
        guard networkEnabled else {
            cancelFavoriteWeatherTasks(except: [])
            return
        }
        if !areFavoritePinsVisible(on: mapView) {
            cancelFavoriteWeatherTasks(except: [])
            return
        }

        let visible = mapView.favoritePinAnnotations
            .filter { mapView.visibleMapRect.contains(MKMapPoint($0.coordinate)) }
        let visibleIDs = Set(visible.map(\.id))

        cancelFavoriteWeatherTasks(except: visibleIDs)

        let now = Date()
        for annotation in visible {
            if let cached = favoriteWeatherCache[annotation.id],
               now.timeIntervalSince(cached.fetchedAt) < favoriteWeatherTTL
            {
                apply(weather: cached.weather, to: annotation, on: mapView)
                continue
            }

            if favoriteWeatherTasks[annotation.id] != nil {
                continue
            }
            startVisibleFavoriteWeatherFetch(
                id: annotation.id,
                latitude: annotation.coordinate.latitude,
                longitude: annotation.coordinate.longitude
            )
        }
    }

    private func startVisibleFavoriteWeatherFetch(id: UUID, latitude: Double, longitude: Double) {
        favoriteWeatherTasks[id] = Task { [weak self] in
            guard let self else { return }
            defer { self.favoriteWeatherTasks[id] = nil }

            do {
                let response = try await weatherService.fetchWeather(lat: latitude, lon: longitude)
                guard !Task.isCancelled else { return }

                let weather = FavoritePinWeather.from(response: response)
                favoriteWeatherCache[id] = CachedFavoritePinWeather(weather: weather, fetchedAt: Date())

                guard let mapView else { return }
                guard let annotation = mapView.favoritePinAnnotation(id: id) else {
                    return
                }
                apply(weather: weather, to: annotation, on: mapView)
            } catch {
                // Keep previous cached weather on failures.
            }
        }
    }

    private func apply(weather: FavoritePinWeather, to annotation: FavoritePinAnnotation, on mapView: MKMapView) {
        annotation.weather = weather
        refreshFavoriteView(for: annotation, on: mapView)
    }

    private func areFavoritePinsVisible(on mapView: MKMapView) -> Bool {
        mapView.region.span.latitudeDelta <= favoritePinsMaxLatitudeDelta
    }

    private func applyFavoritePinVisibility(on mapView: MKMapView) {
        let visible = areFavoritePinsVisible(on: mapView)
        for annotation in mapView.favoritePinAnnotations {
            annotation.isVisibleAtCurrentZoom = visible
            refreshFavoriteView(for: annotation, on: mapView)
        }
    }

    private func cancelFavoriteWeatherTasks(except retainedIDs: Set<UUID>) {
        for id in Array(favoriteWeatherTasks.keys) where !retainedIDs.contains(id) {
            favoriteWeatherTasks[id]?.cancel()
            favoriteWeatherTasks[id] = nil
        }
    }

    private func cancelPreviewTasks() {
        previewWeatherTask?.cancel()
        previewGeocodeTask?.cancel()
        previewWeatherTask = nil
        previewGeocodeTask = nil
    }

    private func removePreviewAnnotation(from mapView: MKMapView) {
        if let previewAnnotation {
            mapView.removeAnnotation(previewAnnotation)
        }
        previewAnnotation = nil
    }

    private func activePreviewMapView(
        for annotation: PreviewPinAnnotation,
        mapView: MKMapView?
    ) -> MKMapView? {
        guard !Task.isCancelled, let mapView else { return nil }
        guard previewAnnotation === annotation else { return nil }
        return mapView
    }

    private func refreshFavoriteView(for annotation: FavoritePinAnnotation, on mapView: MKMapView) {
        mapView.bubbleView(for: annotation)?.configureFavorite(with: annotation)
    }

    private func refreshPreviewView(for annotation: PreviewPinAnnotation, on mapView: MKMapView) {
        mapView.bubbleView(for: annotation)?.configurePreview(with: annotation)
    }

    private func applyOverlay(
        image: UIImage,
        bbox: MapBBox,
        clipsToFinland: Bool = false,
        on mapView: MKMapView
    ) {
        let existing = mapView.overlays.compactMap { $0 as? TemperatureImageOverlay }
        if let current = existing.first(where: { $0.bbox == bbox && $0.clipsToFinland == clipsToFinland }) {
            current.image = image
            if let renderer = mapView.renderer(for: current) {
                renderer.setNeedsDisplay()
            }
            // Drop any other stale overlays (different bbox or clip).
            let stale = existing.filter { $0 !== current }
            if !stale.isEmpty {
                mapView.removeOverlays(stale)
            }
            return
        }
        let new = TemperatureImageOverlay(bbox: bbox, image: image, clipsToFinland: clipsToFinland)
        mapView.addOverlay(new, level: .aboveRoads)
        if !existing.isEmpty {
            mapView.removeOverlays(existing)
        }
    }

    private func applyInitialRegion(on mapView: MKMapView) {
        if let coordinate = preferredCenter {
            mapView.setRegion(
                MKCoordinateRegion(
                    center: coordinate,
                    span: MKCoordinateSpan(latitudeDelta: 1.2, longitudeDelta: 1.2)
                ),
                animated: false
            )
            didCenterOnPreferredLocation = true
        } else {
            mapView.setRegion(
                MKCoordinateRegion(
                    center: CLLocationCoordinate2D(latitude: 64.9, longitude: 25.5),
                    span: MKCoordinateSpan(latitudeDelta: 9.0, longitudeDelta: 11.0)
                ),
                animated: false
            )
            didCenterOnPreferredLocation = false
        }
    }
}
