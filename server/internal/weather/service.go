package weather

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"
)

// Finland coverage bbox — must match the WFS observation query bounds in
// internal/fmi/client.go. Requests outside this box return ErrOutOfCoverage.
const (
	finlandMinLon = 19.0
	finlandMinLat = 59.0
	finlandMaxLon = 32.0
	finlandMaxLat = 71.0
)

var ErrOutOfCoverage = errors.New("location outside coverage area")

type WeatherStore interface {
	NearestStation(ctx context.Context, lat, lon float64) (Station, float64, error)
	LatestObservation(ctx context.Context, fmisid int) (Observation, error)
	GetLatestTemperatureSamplesInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, limit int) ([]TemperatureSample, error)
	GetObservationSamplesAtTimeInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time, limit int) ([]TemperatureSample, error)
	GetHourlyForecastSamplesAtTimeInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time, limit int) ([]TemperatureSample, error)
	StationsInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64) ([]Station, error)
	GetForecasts(ctx context.Context, gridLat, gridLon float64) ([]DailyForecast, error)
	UpsertForecasts(ctx context.Context, forecasts []DailyForecast) error
	GetHourlyForecasts(ctx context.Context, gridLat, gridLon float64, limit int) ([]HourlyForecast, error)
	UpsertHourlyForecasts(ctx context.Context, gridLat, gridLon float64, hourly []HourlyForecast) error
	UpsertClimateNormals(ctx context.Context, normals []ClimateNormal) error
	GetClimateNormals(ctx context.Context, fmisid int, period string) ([]ClimateNormal, error)
	NearestStationWithClimateNormals(ctx context.Context, lat, lon float64, period string) (Station, float64, error)
	GetLeaderboard(ctx context.Context, lat, lon float64, timeframe string) ([]LeaderboardEntry, error)
}

type ForecastFetcher interface {
	FetchForecast(ctx context.Context, lat, lon float64) (ForecastData, error)
	FetchHourlyForecast(ctx context.Context, lat, lon float64, limit int) ([]HourlyForecast, error)
	FetchUVForecast(ctx context.Context, lat, lon float64) ([]UVDataPoint, error)
}

// WMSTileFetcher fetches a single rasterized WMS tile from FMI. The Service
// uses it for the precipitation overlay; it is optional.
type WMSTileFetcher interface {
	FetchWMSTile(ctx context.Context, req WMSTileRequest) ([]byte, error)
}

// GribTemperatureSource reads a gridded temperature field (as a Celsius
// FieldGrid) over a bbox from the gribsvc service, for the forecast temperature
// overlay. A non-zero `at` requests that hour. A nil grid with a nil error is a
// soft miss (file/field not available yet) and the caller falls back to its
// station source. Optional.
type GribTemperatureSource interface {
	Grid(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) (*FieldGrid, time.Time, error)
}

// GribPrecipitationSource reads a gridded precipitation-rate field (as mm/h
// FieldSamples) over a bbox from the gribsvc service, for the 12h precipitation
// forecast overlay. Same `at` and soft-miss semantics as GribTemperatureSource.
// Optional.
type GribPrecipitationSource interface {
	Samples(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) ([]FieldSample, time.Time, error)
}

// WMSTileRequest is the request shape passed to a WMSTileFetcher.
//
// BBox is in lon/lat degrees (WGS84). The fetcher is responsible for
// projecting to whatever CRS the layer requires.
type WMSTileRequest struct {
	Layer  string
	Style  string
	Time   time.Time
	MinLon float64
	MinLat float64
	MaxLon float64
	MaxLat float64
	Width  int
	Height int
}

type Service struct {
	store            WeatherStore
	fmi              ForecastFetcher
	wms              WMSTileFetcher
	forecastCache    *Cache[[]DailyForecast]
	timezoneCache    *Cache[string]
	hourlyCache      *Cache[[]HourlyForecast]
	uvCache          *Cache[[]UVDataPoint]
	precipCache      *Cache[*PrecipitationOverlay]
	leaderboardCache *Cache[[]LeaderboardEntry]

	precipObsLayer  string
	precipFcstLayer string
	precipStyle     string

	gribTemp      GribTemperatureSource
	gribGridCache *Cache[*FieldGrid]

	gribPrecip      GribPrecipitationSource
	gribPrecipCache *Cache[*PrecipitationOverlay]

	gridBackfillMu         sync.Mutex
	gridBackfillInProgress bool
	gridBackfillLastRun    time.Time
}

func NewService(store WeatherStore, fmiClient ForecastFetcher, forecastCacheTTL time.Duration) *Service {
	wms, _ := fmiClient.(WMSTileFetcher)
	return &Service{
		store:            store,
		fmi:              fmiClient,
		wms:              wms,
		forecastCache:    NewCache[[]DailyForecast](forecastCacheTTL),
		timezoneCache:    NewCache[string](forecastCacheTTL),
		hourlyCache:      NewCache[[]HourlyForecast](forecastCacheTTL),
		uvCache:          NewCache[[]UVDataPoint](forecastCacheTTL),
		precipCache:      NewCache[*PrecipitationOverlay](30 * time.Minute),
		leaderboardCache: NewCache[[]LeaderboardEntry](5 * time.Minute),
		gribGridCache:    NewCache[*FieldGrid](10 * time.Minute),
		gribPrecipCache:  NewCache[*PrecipitationOverlay](10 * time.Minute),
	}
}

// SetGribTemperatureSource configures the GRIB-backed temperature field used
// for the map overlay samples. A nil source (the default) keeps the overlay on
// the station-interpolation path.
func (s *Service) SetGribTemperatureSource(src GribTemperatureSource) {
	s.gribTemp = src
}

// SetGribPrecipitationSource configures the GRIB-backed precipitation-rate
// field used for the 12h forecast overlay. A nil source (the default) makes
// GetPrecipitationForecastOverlay report the feature as disabled.
func (s *Service) SetGribPrecipitationSource(src GribPrecipitationSource) {
	s.gribPrecip = src
}

// SetPrecipitationLayers configures the WMS layer names used for the
// precipitation overlay. Both can be empty to disable the feature.
func (s *Service) SetPrecipitationLayers(observationLayer, forecastLayer string) {
	s.precipObsLayer = observationLayer
	s.precipFcstLayer = forecastLayer
}

// SetPrecipitationStyle configures the WMS style passed with each
// precipitation tile request (e.g. "Mobile_dark"). Empty means default.
func (s *Service) SetPrecipitationStyle(style string) {
	s.precipStyle = style
}

func (s *Service) GetWeather(ctx context.Context, lat, lon float64) (*WeatherResponse, error) {
	if lon < finlandMinLon || lon > finlandMaxLon || lat < finlandMinLat || lat > finlandMaxLat {
		return nil, ErrOutOfCoverage
	}

	station, distKM, err := s.store.NearestStation(ctx, lat, lon)
	if err != nil {
		return nil, fmt.Errorf("nearest station: %w", err)
	}

	obs, err := s.store.LatestObservation(ctx, station.FMISID)
	if err != nil {
		return nil, fmt.Errorf("latest observation: %w", err)
	}

	gridLat, gridLon := snapToGrid(lat, lon)
	forecast, forecastTimezone, err := s.getForecast(ctx, gridLat, gridLon)
	if err != nil {
		return nil, fmt.Errorf("forecast: %w", err)
	}
	hourly, err := s.getHourlyForecast(ctx, gridLat, gridLon, 12)
	if err != nil {
		slog.Warn("hourly forecast unavailable", "err", err, "lat", gridLat, "lon", gridLon)
	}

	uvPoints := s.getUVData(ctx, gridLat, gridLon)
	if len(uvPoints) > 0 {
		applyUVToHourly(uvPoints, hourly)
		applyUVToDaily(uvPoints, forecast)
		if err := s.store.UpsertHourlyForecasts(ctx, gridLat, gridLon, hourly); err != nil {
			slog.Warn("failed to persist UV-enriched hourly forecasts", "err", err)
		}
		if err := s.store.UpsertForecasts(ctx, forecast); err != nil {
			slog.Warn("failed to persist UV-enriched daily forecasts", "err", err)
		}
	}

	return &WeatherResponse{
		Current: CurrentWeather{
			Station:     station,
			DistanceKM:  distKM,
			Observation: obs,
		},
		Hourly:   hourly,
		Forecast: forecast,
		Timezone: forecastTimezone,
	}, nil
}

func (s *Service) GetTemperatureSamples(ctx context.Context) (*TemperatureSamplesResponse, error) {
	return s.GetTemperatureSamplesAt(ctx, time.Time{})
}

// GetTemperatureSamplesAt returns temperature samples for the given instant.
// A zero `at` yields the latest available observations (live "now" path).
// A past `at` queries observations near that instant; a future `at` queries
// the hourly forecast snapped to the nearest hour.
func (s *Service) GetTemperatureSamplesAt(ctx context.Context, at time.Time) (*TemperatureSamplesResponse, error) {
	const margin = 0.2
	minLon := finlandMinLon - margin
	minLat := finlandMinLat - margin
	maxLon := finlandMaxLon + margin
	maxLat := finlandMaxLat + margin

	// GRIB is a forecast field, so it's used only for future instants. It carries
	// grid topology, letting the client interpolate with hardware bilinear rather
	// than point IDW. "now" and past instants come from station observations.
	if at.After(time.Now()) {
		if grid := s.gribTemperatureGrid(ctx, minLon, minLat, maxLon, maxLat, at); grid != nil {
			return temperatureGridResponse(grid), nil
		}
	}

	var (
		samples []TemperatureSample
		err     error
	)
	switch {
	case at.IsZero():
		// Live "now": latest station observations.
		samples, err = s.store.GetLatestTemperatureSamplesInBBox(ctx, minLon, minLat, maxLon, maxLat, 350)
	case !at.After(time.Now()):
		// Past: observations near that instant.
		samples, err = s.store.GetObservationSamplesAtTimeInBBox(ctx, minLon, minLat, maxLon, maxLat, at, 350)
	default:
		// Future GRIB miss — fall back to hourly forecasts.
		samples, err = s.store.GetHourlyForecastSamplesAtTimeInBBox(ctx, minLon, minLat, maxLon, maxLat, at, 350)
	}
	if err != nil {
		return nil, fmt.Errorf("temperature samples: %w", err)
	}

	// TODO(grib): remove this station forecast fan-out once the GRIB overlay is
	// validated end to end. GRIB now supplies the dense future field directly,
	// so this only fires when GRIB is unavailable and is the ~200-request burst
	// we built the GRIB service to eliminate. Removing it means dropping
	// triggerForecastGridBackfill / runForecastGridFetch / PrewarmForecastGrid /
	// RunForecastGridPrewarmLoop and the future station fallback below.
	//
	// Trigger backfill before the min-samples gate so the empty/sparse case
	// (0-2 forecast rows) still kicks off a fan-out instead of returning 502
	// with no recovery scheduled. Skip when `at` is beyond the horizon — the
	// fan-out only fetches the next ForecastBackfillHorizon hours, so refilling
	// for a request farther out can never satisfy it.
	now := time.Now()
	horizonLimit := now.Add(time.Duration(ForecastBackfillHorizon) * time.Hour)
	if !at.IsZero() && at.After(now) && !at.After(horizonLimit) && len(samples) < ForecastBackfillThreshold {
		s.triggerForecastGridBackfill(minLon, minLat, maxLon, maxLat)
	}

	if len(samples) < overlayMinSamples {
		return nil, fmt.Errorf("not enough samples")
	}

	minTemp := samples[0].Temperature
	maxTemp := samples[0].Temperature
	dataTime := samples[0].ObservedAt
	for _, sample := range samples[1:] {
		if sample.Temperature < minTemp {
			minTemp = sample.Temperature
		}
		if sample.Temperature > maxTemp {
			maxTemp = sample.Temperature
		}
		if sample.ObservedAt.After(dataTime) {
			dataTime = sample.ObservedAt
		}
	}

	return &TemperatureSamplesResponse{
		DataTime: dataTime.UTC().Truncate(time.Second),
		MinTemp:  minTemp,
		MaxTemp:  maxTemp,
		Samples:  samples,
	}, nil
}

// ForecastBackfillThreshold is the sample count below which a future-time
// request schedules a background grid refill. Exported so the API layer can
// match the predicate when deciding cache policy on sparse responses.
const ForecastBackfillThreshold = 30

// ForecastBackfillHorizon is the maximum number of hours into the future for
// which the backfill prefetches hourly forecasts. Requests for `at` beyond this
// horizon cannot be satisfied by the backfill, so the API layer rejects them.
const ForecastBackfillHorizon = 24

const (
	forecastBackfillCooldown = 5 * time.Minute
	forecastBackfillTimeout  = 2 * time.Minute
	forecastBackfillWorkers  = 6
)

// triggerForecastGridBackfill kicks off a one-shot background fan-out that
// refreshes hourly forecasts for every station in the bbox so that future-time
// scrubbing has a dense sample field. Deduped by an in-process cooldown.
func (s *Service) triggerForecastGridBackfill(minLon, minLat, maxLon, maxLat float64) {
	s.gridBackfillMu.Lock()
	if s.gridBackfillInProgress {
		s.gridBackfillMu.Unlock()
		return
	}
	if time.Since(s.gridBackfillLastRun) < forecastBackfillCooldown {
		s.gridBackfillMu.Unlock()
		return
	}
	s.gridBackfillInProgress = true
	s.gridBackfillMu.Unlock()

	go func() {
		defer s.markBackfillDone()
		ctx, cancel := context.WithTimeout(context.Background(), forecastBackfillTimeout)
		defer cancel()
		s.runForecastGridFetch(ctx, minLon, minLat, maxLon, maxLat)
	}()
}

// PrewarmForecastGrid runs a synchronous Finland-wide forecast fetch for all
// known stations. Intended to be invoked at startup and on a periodic loop so
// the time-scrubber always has a dense field. Skips if a backfill is already
// running. Returns true if stations were found (whether the fetch succeeded
// per-station or not); false when the station table is empty or another
// backfill is already in flight.
func (s *Service) PrewarmForecastGrid(ctx context.Context) bool {
	s.gridBackfillMu.Lock()
	if s.gridBackfillInProgress {
		s.gridBackfillMu.Unlock()
		return false
	}
	s.gridBackfillInProgress = true
	s.gridBackfillMu.Unlock()
	defer s.markBackfillDone()

	const margin = 0.2
	return s.runForecastGridFetch(
		ctx,
		finlandMinLon-margin,
		finlandMinLat-margin,
		finlandMaxLon+margin,
		finlandMaxLat+margin,
	)
}

// RunForecastGridPrewarmLoop runs PrewarmForecastGrid immediately, then on
// `interval`. On a fresh DB the station table may still be empty when the
// loop first runs, so we retry on a short cadence until stations are present.
// Returns when ctx is cancelled.
func (s *Service) RunForecastGridPrewarmLoop(ctx context.Context, interval time.Duration) {
	const emptyRetryInterval = 30 * time.Second
	slog.Info("forecast grid prewarm loop starting", "interval", interval)

	for {
		if s.PrewarmForecastGrid(ctx) {
			break
		}
		select {
		case <-ctx.Done():
			slog.Info("forecast grid prewarm loop stopped")
			return
		case <-time.After(emptyRetryInterval):
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("forecast grid prewarm loop stopped")
			return
		case <-ticker.C:
			s.PrewarmForecastGrid(ctx)
		}
	}
}

func (s *Service) markBackfillDone() {
	s.gridBackfillMu.Lock()
	s.gridBackfillInProgress = false
	s.gridBackfillLastRun = time.Now()
	s.gridBackfillMu.Unlock()
}

func (s *Service) runForecastGridFetch(ctx context.Context, minLon, minLat, maxLon, maxLat float64) bool {
	stations, err := s.store.StationsInBBox(ctx, minLon, minLat, maxLon, maxLat)
	if err != nil {
		slog.Warn("forecast grid fetch: stations lookup failed", "err", err)
		return false
	}
	if len(stations) == 0 {
		return false
	}

	slog.Info("forecast grid fetch: starting", "stations", len(stations))
	start := time.Now()
	sem := make(chan struct{}, forecastBackfillWorkers)
	var wg sync.WaitGroup
	var fetched int64
	var failed int64
	var mu sync.Mutex

	for _, station := range stations {
		sem <- struct{}{}
		wg.Add(1)
		go func(st Station) {
			defer wg.Done()
			defer func() { <-sem }()
			gridLat, gridLon := snapToGrid(st.Lat, st.Lon)
			hourly, err := s.fmi.FetchHourlyForecast(ctx, gridLat, gridLon, ForecastBackfillHorizon)
			if err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			fetchedAt := time.Now()
			for i := range hourly {
				hourly[i].FetchedAt = fetchedAt
			}
			if err := s.store.UpsertHourlyForecasts(ctx, gridLat, gridLon, hourly); err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			mu.Lock()
			fetched++
			mu.Unlock()
		}(station)
	}
	wg.Wait()
	slog.Info("forecast grid fetch: done", "fetched", fetched, "failed", failed, "duration", time.Since(start))
	return true
}

func (s *Service) GetTemperatureOverlay(ctx context.Context, req MapOverlayRequest) (*TemperatureOverlay, error) {
	// Add a small margin so the interpolation near viewport edges has enough support points.
	const marginDeg = 0.2
	minLon := req.MinLon - marginDeg
	minLat := req.MinLat - marginDeg
	maxLon := req.MaxLon + marginDeg
	maxLat := req.MaxLat + marginDeg

	samples, err := s.store.GetLatestTemperatureSamplesInBBox(ctx, minLon, minLat, maxLon, maxLat, 350)
	if err != nil {
		return nil, fmt.Errorf("temperature samples: %w", err)
	}

	overlay, err := RenderTemperatureOverlay(req, samples)
	if err != nil {
		return nil, fmt.Errorf("render overlay: %w", err)
	}
	return overlay, nil
}

func (s *Service) getForecast(ctx context.Context, gridLat, gridLon float64) ([]DailyForecast, string, error) {
	cacheKey := fmt.Sprintf("%.2f,%.2f", gridLat, gridLon)

	if cached, ok := s.forecastCache.Get(cacheKey); ok {
		if hasExpandedForecastData(cached) {
			return cached, s.cachedTimezoneForKey(cacheKey), nil
		}
	}

	forecasts, err := s.store.GetForecasts(ctx, gridLat, gridLon)
	if err == nil && len(forecasts) > 0 && isFresh(forecasts, 3*time.Hour) && hasExpandedForecastData(forecasts) {
		s.forecastCache.Set(cacheKey, forecasts)
		return forecasts, s.cachedTimezoneForKey(cacheKey), nil
	}

	forecastData, err := s.fmi.FetchForecast(ctx, gridLat, gridLon)
	if err != nil {
		return nil, "", err
	}
	forecasts = forecastData.Forecasts
	timezone := normalizePlaceTimezone(forecastData.Timezone)

	if storeErr := s.store.UpsertForecasts(ctx, forecasts); storeErr != nil {
		slog.Warn("failed to store forecasts", "err", storeErr)
	}
	s.forecastCache.Set(cacheKey, forecasts)
	s.timezoneCache.Set(cacheKey, timezone)

	return forecasts, timezone, nil
}

func (s *Service) cachedTimezoneForKey(cacheKey string) string {
	if cached, ok := s.timezoneCache.Get(cacheKey); ok {
		return normalizePlaceTimezone(cached)
	}
	return DefaultPlaceTimezone
}

func normalizePlaceTimezone(value string) string {
	if strings.TrimSpace(value) == "" {
		return DefaultPlaceTimezone
	}
	return value
}

func (s *Service) getHourlyForecast(ctx context.Context, gridLat, gridLon float64, limit int) ([]HourlyForecast, error) {
	cacheKey := fmt.Sprintf("%.2f,%.2f:%d", gridLat, gridLon, limit)
	if cached, ok := s.hourlyCache.Get(cacheKey); ok {
		return cached, nil
	}

	persistedHourly, storeErr := s.store.GetHourlyForecasts(ctx, gridLat, gridLon, limit)
	if storeErr == nil && len(persistedHourly) > 0 && isHourlyFresh(persistedHourly, 90*time.Minute) {
		s.hourlyCache.Set(cacheKey, persistedHourly)
		return persistedHourly, nil
	}

	hourly, err := s.fmi.FetchHourlyForecast(ctx, gridLat, gridLon, limit)
	if err != nil {
		if len(persistedHourly) > 0 {
			slog.Warn("using stale persisted hourly forecast", "err", err, "lat", gridLat, "lon", gridLon)
			s.hourlyCache.Set(cacheKey, persistedHourly)
			return persistedHourly, nil
		}
		return nil, err
	}

	fetchedAt := time.Now()
	for i := range hourly {
		hourly[i].FetchedAt = fetchedAt
	}

	if upsertErr := s.store.UpsertHourlyForecasts(ctx, gridLat, gridLon, hourly); upsertErr != nil {
		slog.Warn("failed to store hourly forecasts", "err", upsertErr)
	}
	s.hourlyCache.Set(cacheKey, hourly)
	return hourly, nil
}

// gribTemperatureGrid returns the GRIB temperature raster over the bbox at the
// hour containing `at`, as a Celsius FieldGrid. Results are cached per hour.
// Returns nil when GRIB is unconfigured, the field isn't available yet, or the
// call fails — callers fall back to their station-based source.
func (s *Service) gribTemperatureGrid(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) *FieldGrid {
	if s.gribTemp == nil {
		return nil
	}

	hour := at.UTC().Truncate(time.Hour)
	cacheKey := fmt.Sprintf("gribgrid:%s", hour.Format(time.RFC3339))
	if cached, ok := s.gribGridCache.Get(cacheKey); ok {
		return cached
	}

	grid, _, err := s.gribTemp.Grid(ctx, minLon, minLat, maxLon, maxLat, hour)
	if err != nil {
		slog.Warn("grib temperature grid failed", "err", err, "at", hour)
		return nil
	}
	if grid == nil || len(grid.Values) == 0 {
		return nil
	}

	s.gribGridCache.Set(cacheKey, grid)
	return grid
}

// temperatureGridResponse builds a samples response carrying the dense GRIB
// raster (Samples left empty; clients use Grid). Min/max are over valid cells.
func temperatureGridResponse(grid *FieldGrid) *TemperatureSamplesResponse {
	minTemp, maxTemp := math.Inf(1), math.Inf(-1)
	for _, v := range grid.Values {
		if v == nil {
			continue
		}
		if *v < minTemp {
			minTemp = *v
		}
		if *v > maxTemp {
			maxTemp = *v
		}
	}
	if math.IsInf(minTemp, 1) {
		minTemp, maxTemp = 0, 0
	}
	return &TemperatureSamplesResponse{
		DataTime: grid.ObservedAt.UTC().Truncate(time.Second),
		MinTemp:  minTemp,
		MaxTemp:  maxTemp,
		Grid:     grid,
	}
}

func snapToGrid(lat, lon float64) (float64, float64) {
	return math.Round(lat*100) / 100, math.Round(lon*100) / 100
}

func isFresh(forecasts []DailyForecast, maxAge time.Duration) bool {
	oldest := forecasts[0].FetchedAt
	for _, f := range forecasts[1:] {
		if f.FetchedAt.Before(oldest) {
			oldest = f.FetchedAt
		}
	}
	return time.Since(oldest) < maxAge
}

func hasExpandedForecastData(forecasts []DailyForecast) bool {
	for _, f := range forecasts {
		// TempAvg is derived from Temperature and should be present for any day with temp data.
		// If it's nil across all days, rows are likely from pre-migration/pre-rollout data.
		if f.TempAvg != nil {
			return true
		}
	}
	return false
}

func (s *Service) getUVData(ctx context.Context, gridLat, gridLon float64) []UVDataPoint {
	cacheKey := fmt.Sprintf("uv:%.2f,%.2f", gridLat, gridLon)
	if cached, ok := s.uvCache.Get(cacheKey); ok {
		return cached
	}

	points, err := s.fmi.FetchUVForecast(ctx, gridLat, gridLon)
	if err != nil {
		slog.Warn("UV forecast fetch failed", "err", err)
		return nil
	}
	slog.Info("fetched UV forecast from FMI", "lat", gridLat, "lon", gridLon, "points", len(points), "data", points)
	if len(points) > 0 {
		s.uvCache.Set(cacheKey, points)
	}
	return points
}

func applyUVToHourly(uvPoints []UVDataPoint, hourly []HourlyForecast) {
	uvByHour := make(map[int64]float64, len(uvPoints))
	for _, p := range uvPoints {
		uvByHour[p.Time.Truncate(time.Hour).Unix()] = p.UVCumulated
	}
	for i := range hourly {
		if uv, ok := uvByHour[hourly[i].Time.Truncate(time.Hour).Unix()]; ok {
			hourly[i].UVCumulated = &uv
		}
	}
}

func applyUVToDaily(uvPoints []UVDataPoint, forecasts []DailyForecast) {
	type dailyUV struct {
		sum   float64
		count int
	}
	byDate := make(map[string]*dailyUV)
	for _, p := range uvPoints {
		date := p.Time.UTC().Format("2006-01-02")
		d, ok := byDate[date]
		if !ok {
			d = &dailyUV{}
			byDate[date] = d
		}
		d.sum += p.UVCumulated
		d.count++
	}
	for i := range forecasts {
		date := forecasts[i].Date.UTC().Format("2006-01-02")
		if d, ok := byDate[date]; ok && d.count > 0 {
			avg := d.sum / float64(d.count)
			forecasts[i].UVIndexAvg = &avg
		}
	}
}

const maxClimateNormalsDistanceKm = 50.0

func (s *Service) GetClimateNormals(ctx context.Context, lat, lon float64, currentTemp *float64) (*Station, float64, []ClimateNormal, InterpolatedNormal, error) {
	station, distKm, err := s.store.NearestStationWithClimateNormals(ctx, lat, lon, "1991-2020")
	if err != nil {
		return nil, 0, nil, InterpolatedNormal{}, fmt.Errorf("nearest station with climate normals: %w", err)
	}

	if distKm > maxClimateNormalsDistanceKm {
		return nil, 0, nil, InterpolatedNormal{}, nil
	}

	normals, err := s.store.GetClimateNormals(ctx, station.FMISID, "1991-2020")
	if err != nil {
		return nil, 0, nil, InterpolatedNormal{}, fmt.Errorf("get climate normals: %w", err)
	}

	today := InterpolateNormals(normals, time.Now().UTC(), currentTemp)
	return &station, distKm, normals, today, nil
}

func (s *Service) GetLeaderboard(ctx context.Context, lat, lon float64, timeframe string) ([]LeaderboardEntry, error) {
	// Snap to 1-degree grid for cache key — distance doesn't need high precision
	gridLat := math.Round(lat)
	gridLon := math.Round(lon)
	cacheKey := fmt.Sprintf("lb:%.0f,%.0f:%s", gridLat, gridLon, timeframe)

	if cached, ok := s.leaderboardCache.Get(cacheKey); ok {
		return cached, nil
	}

	entries, err := s.store.GetLeaderboard(ctx, lat, lon, timeframe)
	if err != nil {
		return nil, fmt.Errorf("leaderboard: %w", err)
	}

	s.leaderboardCache.Set(cacheKey, entries)
	return entries, nil
}

func isHourlyFresh(hourly []HourlyForecast, maxAge time.Duration) bool {
	oldest := hourly[0].FetchedAt
	if oldest.IsZero() {
		return false
	}
	for _, h := range hourly[1:] {
		if h.FetchedAt.IsZero() {
			return false
		}
		if h.FetchedAt.Before(oldest) {
			oldest = h.FetchedAt
		}
	}
	return time.Since(oldest) < maxAge
}
