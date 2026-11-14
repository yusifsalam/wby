package weather

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"slices"
	"strings"
	"sync/atomic"
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

func inCoverage(lat, lon float64) bool {
	return lon >= finlandMinLon && lon <= finlandMaxLon && lat >= finlandMinLat && lat <= finlandMaxLat
}

// ErrNoStation is returned by store lookups that found no matching station.
var ErrNoStation = errors.New("no station found")

// ErrForecastGridUnavailable is returned for a future-time temperature request
// whose hourly GRIB frame isn't in the warmed cache.
var ErrForecastGridUnavailable = errors.New("forecast grid not available")

type WeatherStore interface {
	NearestStation(ctx context.Context, lat, lon float64) (Station, float64, error)
	NearestStations(ctx context.Context, lat, lon, radiusKm float64, limit int) ([]StationDistance, error)
	LatestObservation(ctx context.Context, fmisid int) (Observation, error)
	LatestObservations(ctx context.Context, fmisids []int) (map[int]Observation, error)
	ObservedTemperatureRange(ctx context.Context, fmisid int, from, to time.Time) (low, high *float64, err error)
	GetLatestTemperatureSamplesInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, limit int) ([]TemperatureSample, error)
	GetObservationSamplesAtTimeInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time, limit int) ([]TemperatureSample, error)
	GetForecasts(ctx context.Context, gridLat, gridLon float64, from time.Time) ([]DailyForecast, error)
	UpsertForecasts(ctx context.Context, forecasts []DailyForecast) error
	GetHourlyForecasts(ctx context.Context, gridLat, gridLon float64, from time.Time, limit int) ([]HourlyForecast, error)
	UpsertHourlyForecasts(ctx context.Context, gridLat, gridLon float64, hourly []HourlyForecast) error
	UpsertClimateNormals(ctx context.Context, normals []ClimateNormal) error
	GetClimateNormals(ctx context.Context, fmisid int, period string) ([]ClimateNormal, error)
	NearestStationWithClimateNormals(ctx context.Context, lat, lon float64, period string) (Station, float64, error)
	DailyClimateNormalsCandidates(ctx context.Context, lat, lon float64, period string, month, day int, radiusKm float64) ([]DailyNormalsCandidate, error)
	GetDailyClimateNormals(ctx context.Context, fmisid int, period string) ([]DailyClimateNormal, error)
	UpsertDailyClimateNormals(ctx context.Context, normals []DailyClimateNormal) error
	GetLeaderboard(ctx context.Context, lat, lon float64, timeframe string) ([]LeaderboardEntry, error)
}

type ForecastFetcher interface {
	FetchForecast(ctx context.Context, lat, lon float64) (ForecastData, error)
	FetchHourlyForecast(ctx context.Context, lat, lon float64, start time.Time, hours int) ([]HourlyForecast, error)
	FetchUVForecast(ctx context.Context, lat, lon float64, start time.Time) ([]UVDataPoint, error)
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
	// GridSeries fetches all requested hours in one pass over the GRIB file
	// (used by cache warming). Hours the file lacks are absent from the result;
	// a soft miss returns an empty slice and nil error.
	GridSeries(ctx context.Context, minLon, minLat, maxLon, maxLat float64, times []time.Time) ([]*FieldGrid, error)
}

// GribPrecipitationSource reads a gridded precipitation-rate field (as a mm/h
// FieldGrid) over a bbox from the gribsvc service, for the precipitation
// forecast overlay. Same `at` and soft-miss semantics as GribTemperatureSource.
// Optional.
type GribPrecipitationSource interface {
	Grid(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) (*FieldGrid, time.Time, error)
	GridSeries(ctx context.Context, minLon, minLat, maxLon, maxLat float64, times []time.Time) ([]*FieldGrid, error)
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
	gribPrecipCache *Cache[*PrecipitationForecastGrid]

	radarPrecip      RadarPrecipitationSource
	radarPrecipCache *Cache[*PrecipitationForecastGrid]

	precipHistory     PrecipitationHistoryFetcher
	precipToDateCache *Cache[*PrecipitationObservations]

	forecastFlight flightGroup[forecastResult]
	hourlyFlight   flightGroup[[]HourlyForecast]
	uvFlight       flightGroup[[]UVDataPoint]
	precipFlight   flightGroup[*PrecipitationOverlay]

	precipToDateFlight flightGroup[*PrecipitationObservations]

	gribFile            string
	nowcastFailingSince atomic.Int64
	gribWarmedAt        atomic.Int64
	newestRadarGridAt   atomic.Int64
}

// GribWarmedAt is when a warm last stored GRIB grids; zero when none has
// since the process started.
func (s *Service) GribWarmedAt() time.Time {
	return unixNanoTime(s.gribWarmedAt.Load())
}

// NewestRadarGridAt is the valid time of the newest radar frame held for
// serving; zero when none is.
func (s *Service) NewestRadarGridAt() time.Time {
	return unixNanoTime(s.newestRadarGridAt.Load())
}

func unixNanoTime(ns int64) time.Time {
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

// Caps on the request-keyed caches. A grid cell's hourly forecast is the
// largest entry at roughly 100 KB; a precipitation tile is a PNG of up to
// 1600x1600 pixels.
const (
	maxCachedCells = 1000
	maxCachedTiles = 128
)

// persistTimeout bounds a write that outlives the request that caused it.
const persistTimeout = 10 * time.Second

// upstreamRetryAfter is how long a stale or empty result stands in for a
// failed upstream fetch before the next request tries the upstream again.
const upstreamRetryAfter = time.Minute

type forecastResult struct {
	forecasts []DailyForecast
	timezone  string
}

func NewService(store WeatherStore, fmiClient ForecastFetcher, forecastCacheTTL time.Duration) *Service {
	wms, _ := fmiClient.(WMSTileFetcher)
	precipHistory, _ := fmiClient.(PrecipitationHistoryFetcher)
	return &Service{
		store:             store,
		fmi:               fmiClient,
		wms:               wms,
		precipHistory:     precipHistory,
		precipToDateCache: NewCache[*PrecipitationObservations](time.Hour).Limit(maxCachedCells / 2),
		forecastCache:     NewCache[[]DailyForecast](forecastCacheTTL).Limit(maxCachedCells),
		timezoneCache:     NewCache[string](forecastCacheTTL).Limit(maxCachedCells),
		hourlyCache:       NewCache[[]HourlyForecast](forecastCacheTTL).Limit(maxCachedCells),
		uvCache:           NewCache[[]UVDataPoint](forecastCacheTTL).Limit(maxCachedCells),
		precipCache:       NewCache[*PrecipitationOverlay](30 * time.Minute).Limit(maxCachedTiles),
		leaderboardCache:  NewCache[[]LeaderboardEntry](5 * time.Minute).Limit(maxCachedCells / 2),
		gribGridCache:     NewCache[*FieldGrid](0),
		gribPrecipCache:   NewCache[*PrecipitationForecastGrid](0),
		radarPrecipCache:  NewCache[*PrecipitationForecastGrid](radarGridCacheTTL),
	}
}

// SetGribTemperatureSource configures the GRIB-backed temperature field used
// for the map overlay samples. A nil source (the default) keeps the overlay on
// the station-interpolation path.
func (s *Service) SetGribTemperatureSource(src GribTemperatureSource) {
	s.gribTemp = src
}

// SetGribPrecipitationSource configures the GRIB-backed precipitation-rate
// field used for the precipitation forecast overlay. A nil source (the default) makes
// GetPrecipitationForecastOverlay report the feature as disabled.
func (s *Service) SetGribPrecipitationSource(src GribPrecipitationSource) {
	s.gribPrecip = src
}

// SetGribFile names the GRIB file on disk that the temperature and
// precipitation sources read, so warming can tell how old it is.
func (s *Service) SetGribFile(path string) {
	s.gribFile = path
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
	if !inCoverage(lat, lon) {
		return nil, ErrOutOfCoverage
	}

	station, distKM, obs, err := s.currentConditions(ctx, lat, lon)
	if err != nil {
		return nil, err
	}

	gridLat, gridLon := snapToGrid(lat, lon)
	forecast, forecastTimezone, err := s.getForecast(ctx, gridLat, gridLon)
	if err != nil {
		return nil, fmt.Errorf("forecast: %w", err)
	}
	hourly, err := s.getHourlyForecast(ctx, gridLat, gridLon, forecastTimezone)
	if err != nil {
		slog.Warn("hourly forecast unavailable", "err", err, "lat", gridLat, "lon", gridLon)
	}

	loc := placeLocation(forecastTimezone)
	uvPoints, uvFresh := s.getUVData(ctx, gridLat, gridLon, forecastTimezone)
	if len(uvPoints) > 0 {
		hourly = slices.Clone(hourly)
		forecast = slices.Clone(forecast)
		applyUVToHourly(uvPoints, hourly)
		applyUVToDaily(uvPoints, forecast, loc)
		if uvFresh {
			persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
			if err := s.store.UpsertHourlyForecasts(persistCtx, gridLat, gridLon, hourly); err != nil {
				slog.Warn("failed to persist UV-enriched hourly forecasts", "err", err)
			}
			if err := s.store.UpsertForecasts(persistCtx, forecast); err != nil {
				slog.Warn("failed to persist UV-enriched daily forecasts", "err", err)
			}
			cancel()
		}
	}

	forecast = widenWithObservedRange(forecast, time.Now(), loc, func(from, to time.Time) (*float64, *float64, error) {
		return s.store.ObservedTemperatureRange(ctx, station.FMISID, from, to)
	})

	return &WeatherResponse{
		Current: CurrentWeather{
			Station:     station,
			DistanceKM:  distKM,
			Observation: obs,
		},
		Hourly:            upcomingHours(hourly, time.Now(), weatherResponseHourlyHours),
		Forecast:          forecast,
		UV:                uvPoints,
		Timezone:          forecastTimezone,
		ForecastFetchedAt: oldestFetchedAt(forecast, hourly),
	}, nil
}

// oldestFetchedAt is the earliest time any of the given forecast rows was
// fetched from FMI, or zero when none carries a fetch time.
func oldestFetchedAt(daily []DailyForecast, hourly []HourlyForecast) time.Time {
	var oldest time.Time
	note := func(t time.Time) {
		if !t.IsZero() && (oldest.IsZero() || t.Before(oldest)) {
			oldest = t
		}
	}
	for _, f := range daily {
		note(f.FetchedAt)
	}
	for _, h := range hourly {
		note(h.FetchedAt)
	}
	return oldest
}

// weatherResponseHourlyHours caps the hourly list embedded in GetWeather;
// the full window, including today's elapsed hours, is served by
// GetHourlyForecast.
const weatherResponseHourlyHours = 12

// upcomingHours returns up to n entries from the current hour onwards.
func upcomingHours(hourly []HourlyForecast, now time.Time, n int) []HourlyForecast {
	cutoff := now.Truncate(time.Hour)
	start := 0
	for start < len(hourly) && hourly[start].Time.Before(cutoff) {
		start++
	}
	return hourly[start:min(len(hourly), start+n)]
}

func (s *Service) GetHourlyForecast(ctx context.Context, lat, lon float64) (*HourlyForecastResponse, error) {
	if !inCoverage(lat, lon) {
		return nil, ErrOutOfCoverage
	}

	gridLat, gridLon := snapToGrid(lat, lon)
	_, timezone, err := s.getForecast(ctx, gridLat, gridLon)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("forecast: %w", err)
		}
		slog.Warn("daily forecast unavailable, using the known place timezone", "err", err, "lat", gridLat, "lon", gridLon)
		timezone = s.cachedTimezoneForKey(fmt.Sprintf("%.2f,%.2f", gridLat, gridLon))
	}
	hourly, err := s.getHourlyForecast(ctx, gridLat, gridLon, timezone)
	if err != nil {
		return nil, fmt.Errorf("hourly forecast: %w", err)
	}
	fetchedAt := oldestFetchedAt(nil, hourly)
	if uvPoints, _ := s.getUVData(ctx, gridLat, gridLon, timezone); len(uvPoints) > 0 {
		hourly = slices.Clone(hourly)
		applyUVToHourly(uvPoints, hourly)
	}
	return &HourlyForecastResponse{Hourly: hourly, Timezone: timezone, FetchedAt: fetchedAt}, nil
}

// currentConditions picks the station representing lat/lon (see
// station_pick.go) and composes its observation. With no candidate inside the
// search radius it falls back to the nearest station anywhere, unmerged.
func (s *Service) currentConditions(ctx context.Context, lat, lon float64) (Station, float64, Observation, error) {
	candidates, err := s.store.NearestStations(ctx, lat, lon, stationSearchRadiusKm, stationCandidateLimit)
	if err != nil {
		return Station{}, 0, Observation{}, fmt.Errorf("nearest stations: %w", err)
	}
	if len(candidates) > 0 {
		ids := make([]int, len(candidates))
		for i, c := range candidates {
			ids[i] = c.FMISID
		}
		obs, err := s.store.LatestObservations(ctx, ids)
		if err != nil {
			return Station{}, 0, Observation{}, fmt.Errorf("latest observations: %w", err)
		}
		if picked, merged, ok := pickStation(candidates, obs, time.Now()); ok {
			return picked.Station, picked.DistanceKM, merged, nil
		}
	}

	station, distKM, err := s.store.NearestStation(ctx, lat, lon)
	if err != nil {
		return Station{}, 0, Observation{}, fmt.Errorf("nearest station: %w", err)
	}
	obs, err := s.store.LatestObservation(ctx, station.FMISID)
	if err != nil {
		return Station{}, 0, Observation{}, fmt.Errorf("latest observation: %w", err)
	}
	return station, distKM, withoutExpiredValues(obs, time.Now()), nil
}

func (s *Service) GetTemperatureSamples(ctx context.Context) (*TemperatureSamplesResponse, error) {
	return s.GetTemperatureSamplesAt(ctx, time.Time{})
}

// GetTemperatureSamplesAt returns temperature samples for the given instant.
// A zero `at` yields the latest available observations (live "now" path).
// A past `at` queries observations near that instant. Any instant from the
// start of the current hour forward is served from the warmed GRIB grid
// cache; a future `at` whose frame isn't cached returns
// ErrForecastGridUnavailable rather than extracting on demand.
func (s *Service) GetTemperatureSamplesAt(ctx context.Context, at time.Time) (*TemperatureSamplesResponse, error) {
	const margin = 0.2
	minLon := finlandMinLon - margin
	minLat := finlandMinLat - margin
	maxLon := finlandMaxLon + margin
	maxLat := finlandMaxLat + margin

	// GRIB is a forecast field covering the current hour onward. Use it for any
	// instant from the start of the current hour forward: it carries grid topology,
	// letting the client interpolate with hardware bilinear rather than point IDW,
	// so the "now" overlay is gridded like the forecast frames. Earlier (past)
	// instants fall through to station observations below.
	now := time.Now()
	if !at.Before(now.UTC().Truncate(time.Hour)) {
		if grid := s.gribTemperatureGrid(at); grid != nil {
			return temperatureGridResponse(grid), nil
		}
		if at.After(now) {
			return nil, ErrForecastGridUnavailable
		}
	}

	var (
		samples []TemperatureSample
		err     error
	)
	if at.IsZero() {
		samples, err = s.store.GetLatestTemperatureSamplesInBBox(ctx, minLon, minLat, maxLon, maxLat, 350)
	} else {
		samples, err = s.store.GetObservationSamplesAtTimeInBBox(ctx, minLon, minLat, maxLon, maxLat, at, 350)
	}
	if err != nil {
		return nil, fmt.Errorf("temperature samples: %w", err)
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

// MapForecastHorizon is the furthest-future hour the map forecast exposes and
// accepts. Backed by the GRIB temperature field (Harmonie surface run covers
// ~60h ahead). Both the frame manifest window and the samples handler's `at`
// cap derive from it.
const MapForecastHorizon = 48

// warmHorizonSlack is how many hours past a layer's horizon the warm passes
// extract, covering the hour rollover between passes.
const warmHorizonSlack = 2

// warmSeriesChunkHours caps the frames per GridSeries call; gribsvc holds every
// requested frame in memory before answering.
const warmSeriesChunkHours = 12

// radarGridCacheTTL bounds the radar and nowcast frame caches, which gain a key
// every 5 minutes. GRIB grid caches never expire; the warm pass prunes them.
const radarGridCacheTTL = 75 * time.Minute

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

	overlay, err := RenderTemperatureOverlay(ctx, req, samples)
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

	res, _, err := s.forecastFlight.Do(ctx, cacheKey, func(ctx context.Context) (forecastResult, error) {
		return s.loadForecast(ctx, cacheKey, gridLat, gridLon)
	})
	return res.forecasts, res.timezone, err
}

// loadForecast returns the persisted daily forecast when it is fresh, and
// otherwise fetches it from FMI. When FMI fails or returns nothing, persisted
// rows of any age are served instead.
func (s *Service) loadForecast(ctx context.Context, cacheKey string, gridLat, gridLon float64) (forecastResult, error) {
	persistedTimezone := s.cachedTimezoneForKey(cacheKey)
	persisted, storeErr := s.store.GetForecasts(ctx, gridLat, gridLon, startOfLocalDay(time.Now(), persistedTimezone))
	usable := storeErr == nil && len(persisted) > 0 && hasExpandedForecastData(persisted)
	if usable && isFresh(persisted, 3*time.Hour) {
		s.forecastCache.Set(cacheKey, persisted)
		return forecastResult{persisted, persistedTimezone}, nil
	}

	forecastData, err := s.fmi.FetchForecast(ctx, gridLat, gridLon)
	if (err != nil || len(forecastData.Forecasts) == 0) && usable {
		slog.Warn("using stale persisted daily forecast", "err", err, "lat", gridLat, "lon", gridLon,
			"fetched_at", oldestFetchedAt(persisted, nil))
		s.forecastCache.SetTTL(cacheKey, persisted, upstreamRetryAfter)
		return forecastResult{persisted, persistedTimezone}, nil
	}
	if err != nil {
		return forecastResult{}, err
	}
	forecasts := forecastData.Forecasts
	timezone := normalizePlaceTimezone(forecastData.Timezone)

	if storeErr := s.store.UpsertForecasts(ctx, forecasts); storeErr != nil {
		slog.Warn("failed to store forecasts", "err", storeErr)
	}
	s.forecastCache.Set(cacheKey, forecasts)
	s.timezoneCache.Set(cacheKey, timezone)

	return forecastResult{forecasts, timezone}, nil
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

// getHourlyForecast returns the hourly window from the start of the local
// day, so today's elapsed hours are included.
func (s *Service) getHourlyForecast(ctx context.Context, gridLat, gridLon float64, timezone string) ([]HourlyForecast, error) {
	cacheKey := fmt.Sprintf("%.2f,%.2f", gridLat, gridLon)
	if cached, ok := s.hourlyCache.Get(cacheKey); ok {
		return cached, nil
	}

	hourly, _, err := s.hourlyFlight.Do(ctx, cacheKey, func(ctx context.Context) ([]HourlyForecast, error) {
		return s.loadHourlyForecast(ctx, cacheKey, gridLat, gridLon, timezone)
	})
	return hourly, err
}

func (s *Service) loadHourlyForecast(ctx context.Context, cacheKey string, gridLat, gridLon float64, timezone string) ([]HourlyForecast, error) {
	from := startOfLocalDay(time.Now(), timezone)
	persistedHourly, storeErr := s.store.GetHourlyForecasts(ctx, gridLat, gridLon, from, HourlyForecastHours)
	if storeErr == nil && len(persistedHourly) > 0 && isHourlyFresh(persistedHourly, 90*time.Minute) {
		s.hourlyCache.Set(cacheKey, persistedHourly)
		return persistedHourly, nil
	}

	hourly, err := s.fmi.FetchHourlyForecast(ctx, gridLat, gridLon, from, HourlyForecastHours)
	if err != nil {
		if len(persistedHourly) > 0 {
			slog.Warn("using stale persisted hourly forecast", "err", err, "lat", gridLat, "lon", gridLon,
				"fetched_at", oldestFetchedAt(nil, persistedHourly))
			s.hourlyCache.SetTTL(cacheKey, persistedHourly, upstreamRetryAfter)
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

// gribTemperatureGrid returns the cached GRIB temperature raster (Celsius
// FieldGrid) for the hour containing `at`, or nil when GRIB is unconfigured or
// the hour hasn't been warmed.
func (s *Service) gribTemperatureGrid(at time.Time) *FieldGrid {
	if s.gribTemp == nil {
		return nil
	}
	if cached, ok := s.gribGridCache.Get(gribTempGridKey(at.UTC().Truncate(time.Hour))); ok {
		return cached
	}
	return nil
}

// fetchTemperatureGrid extracts one hour over the bbox from gribsvc and caches
// it. A nil result is a soft miss or a failure.
func (s *Service) fetchTemperatureGrid(ctx context.Context, minLon, minLat, maxLon, maxLat float64, hour time.Time) *FieldGrid {
	grid, _, err := s.gribTemp.Grid(ctx, minLon, minLat, maxLon, maxLat, hour)
	if err != nil {
		slog.Warn("grib temperature grid failed", "err", err, "at", hour)
		return nil
	}
	if grid == nil || len(grid.Values) == 0 {
		return nil
	}
	s.gribGridCache.Set(gribTempGridKey(hour), grid)
	return grid
}

// WarmTemperatureGrids pre-populates the GRIB temperature grid cache for every
// hourly map frame (the current hour through MapForecastHorizon plus
// warmHorizonSlack), over the same fixed Finland extent GetTemperatureSamplesAt
// uses. It is the only path that extracts from gribsvc; it runs at startup and
// after each GRIB download attempt. All frames come from a single GridSeries
// call (one pass over the GRIB file); if that fails or returns nothing,
// warming falls back to one extract per hour. The cache key is hour-only, so
// warming here satisfies every client bbox.
func (s *Service) WarmTemperatureGrids(ctx context.Context) {
	if s.gribTemp == nil || ctx.Err() != nil {
		return
	}
	start := time.Now()
	hours := warmHours(time.Now().UTC().Truncate(time.Hour), MapForecastHorizon+warmHorizonSlack)
	warmed := s.warmTemperatureHours(ctx, hours)
	pruneWarmCache(s.gribGridCache, gribTempGridKey, hours)
	slog.Info("warmed grib temperature grids",
		"frames", warmed,
		"horizon_hours", MapForecastHorizon,
		"duration", time.Since(start),
	)
}

func (s *Service) warmTemperatureHours(ctx context.Context, hours []time.Time) int {
	if s.gribTemp == nil || len(hours) == 0 {
		return 0
	}

	const margin = 0.2
	minLon := finlandMinLon - margin
	minLat := finlandMinLat - margin
	maxLon := finlandMaxLon + margin
	maxLat := finlandMaxLat + margin

	return warmGridSeries(ctx, "temperature", hours,
		func(ctx context.Context, times []time.Time) ([]*FieldGrid, error) {
			return s.gribTemp.GridSeries(ctx, minLon, minLat, maxLon, maxLat, times)
		},
		func(grid *FieldGrid) {
			s.gribGridCache.Set(gribTempGridKey(grid.ObservedAt.UTC().Truncate(time.Hour)), grid)
		},
		func(ctx context.Context, at time.Time) bool {
			return s.fetchTemperatureGrid(ctx, minLon, minLat, maxLon, maxLat, at) != nil
		},
	)
}

// WarmGribGrids warms both GRIB layers for one refresh cycle. The first chunk
// of each layer goes first, precipitation ahead of temperature, so the frames a
// scrubber opens on are live within one series call per layer while the rest
// of the horizon fills in behind them.
func (s *Service) WarmGribGrids(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if age, stale := s.gribFileAge(); stale {
		s.gribGridCache.DeleteIf(func(string) bool { return true })
		s.gribPrecipCache.DeleteIf(func(string) bool { return true })
		slog.Error("grib file too old, dropped its grids", "file", s.gribFile, "age", age.Round(time.Minute))
		return
	}
	start := time.Now()
	base := time.Now().UTC().Truncate(time.Hour)
	precipHours := warmHours(base, PrecipForecastHorizon+warmHorizonSlack)
	tempHours := warmHours(base, MapForecastHorizon+warmHorizonSlack)
	head := func(hours []time.Time) []time.Time { return hours[:min(warmSeriesChunkHours, len(hours))] }
	tail := func(hours []time.Time) []time.Time { return hours[min(warmSeriesChunkHours, len(hours)):] }

	precip := s.warmPrecipitationHours(ctx, head(precipHours))
	temp := s.warmTemperatureHours(ctx, head(tempHours))
	precip += s.warmPrecipitationHours(ctx, tail(precipHours))
	temp += s.warmTemperatureHours(ctx, tail(tempHours))

	if s.gribPrecip != nil {
		pruneWarmCache(s.gribPrecipCache, gribPrecipGridKey, precipHours)
	}
	if s.gribTemp != nil {
		pruneWarmCache(s.gribGridCache, gribTempGridKey, tempHours)
	}
	s.gribGridCache.DeleteFunc(func(_ string, grid *FieldGrid) bool { return gribGridExpired(grid, start) })
	s.gribPrecipCache.DeleteFunc(func(_ string, grid *PrecipitationForecastGrid) bool { return gribGridExpired(grid.Grid, start) })

	if precip+temp > 0 {
		s.gribWarmedAt.Store(time.Now().UnixNano())
	}
	log := slog.Info
	if (s.gribPrecip != nil && precip == 0) || (s.gribTemp != nil && temp == 0) {
		log = slog.Warn
	}
	log("warmed grib grids",
		"precipitation_frames", precip,
		"temperature_frames", temp,
		"duration", time.Since(start),
	)
}

// gribMaxAge bounds how old GRIB data may get before it stops being served:
// both the downloaded file and a grid a later warm failed to re-read.
const gribMaxAge = 6 * time.Hour

// gribFileAge reports how long ago the GRIB file was downloaded and whether
// that exceeds gribMaxAge. A file that is not configured or not on disk is
// not stale.
func (s *Service) gribFileAge() (time.Duration, bool) {
	if s.gribFile == "" {
		return 0, false
	}
	info, err := os.Stat(s.gribFile)
	if err != nil {
		return 0, false
	}
	age := time.Since(info.ModTime())
	return age, age > gribMaxAge
}

func gribGridExpired(grid *FieldGrid, now time.Time) bool {
	return grid != nil && !grid.FetchedAt.IsZero() && now.Sub(grid.FetchedAt) > gribMaxAge
}

func gribTempGridKey(hour time.Time) string {
	return "gribgrid:" + hour.Format(time.RFC3339)
}

// warmHours lists the hourly frame times from base through base+horizon inclusive.
func warmHours(base time.Time, horizon int) []time.Time {
	hours := make([]time.Time, 0, horizon+1)
	for i := 0; i <= horizon; i++ {
		hours = append(hours, base.Add(time.Duration(i)*time.Hour))
	}
	return hours
}

// warmGridSeries fills a grid cache for hours in chunks of warmSeriesChunkHours,
// one GridSeries call per chunk. A chunk whose series call fails or returns
// nothing is fetched hour by hour, except after a timeout: gribsvc is still
// working on the abandoned request. Returns the number of frames stored.
func warmGridSeries(
	ctx context.Context,
	name string,
	hours []time.Time,
	series func(context.Context, []time.Time) ([]*FieldGrid, error),
	store func(*FieldGrid),
	fetch func(context.Context, time.Time) bool,
) int {
	warmed := 0
	for start := 0; start < len(hours); start += warmSeriesChunkHours {
		if ctx.Err() != nil {
			return warmed
		}
		chunk := hours[start:min(start+warmSeriesChunkHours, len(hours))]

		grids, err := series(ctx, chunk)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				slog.Warn("grib "+name+" series timed out, skipping chunk", "err", err, "from", chunk[0])
				continue
			}
			slog.Warn("grib "+name+" series failed, warming per hour", "err", err, "from", chunk[0])
		}
		if len(grids) > 0 {
			for _, grid := range grids {
				if grid == nil || len(grid.Values) == 0 {
					continue
				}
				store(grid)
				warmed++
			}
			continue
		}
		for _, at := range chunk {
			if ctx.Err() != nil {
				return warmed
			}
			if fetch(ctx, at) {
				warmed++
			}
		}
	}
	return warmed
}

// pruneWarmCache drops cache entries for hours outside the warmed window.
func pruneWarmCache[V any](cache *Cache[V], key func(time.Time) string, hours []time.Time) {
	keep := make(map[string]struct{}, len(hours))
	for _, at := range hours {
		keep[key(at)] = struct{}{}
	}
	cache.DeleteIf(func(k string) bool {
		_, ok := keep[k]
		return !ok
	})
}

// pruneWarmCachePrefix drops entries under one key prefix that fall outside the
// warmed window, leaving other key families in the same cache untouched. The
// radar and nowcast frames share a cache, so each prunes only its own.
func pruneWarmCachePrefix[V any](cache *Cache[V], prefix string, key func(time.Time) string, frames []time.Time) {
	keep := make(map[string]struct{}, len(frames))
	for _, at := range frames {
		keep[key(at)] = struct{}{}
	}
	cache.DeleteIf(func(k string) bool {
		if !strings.HasPrefix(k, prefix) {
			return false
		}
		_, ok := keep[k]
		return !ok
	})
}

// temperatureGridResponse builds a samples response carrying the dense GRIB
// raster (Samples left empty; clients use Grid). Min/max are over valid cells.
func temperatureGridResponse(grid *FieldGrid) *TemperatureSamplesResponse {
	minTemp, maxTemp := grid.GridRange()
	return &TemperatureSamplesResponse{
		DataTime: grid.ObservedAt.UTC().Truncate(time.Second),
		MinTemp:  minTemp,
		MaxTemp:  maxTemp,
		Grid:     grid,
	}
}

// widenWithObservedRange extends the high/low of forecast days already underway
// with what the station has observed so far. Forecast days are UTC buckets
// starting at the current hour, so today's range alone only covers the hours
// still ahead.
// widenWithObservedRange folds the observed temperature range of each day's
// elapsed hours into its forecast high/low. Date is a calendar date; the
// day's span runs from its local midnight in loc.
func widenWithObservedRange(forecast []DailyForecast, now time.Time, loc *time.Location, observed func(from, to time.Time) (low, high *float64, err error)) []DailyForecast {
	out := slices.Clone(forecast)
	for i := range out {
		start := time.Date(out[i].Date.Year(), out[i].Date.Month(), out[i].Date.Day(), 0, 0, 0, 0, loc)
		if !start.Before(now) {
			continue
		}
		end := start.AddDate(0, 0, 1)
		if end.After(now) {
			end = now
		}
		low, high, err := observed(start, end)
		if err != nil {
			slog.Warn("observed temperature range unavailable", "err", err, "date", start.Format("2006-01-02"))
			return forecast
		}
		out[i].TempLow = pickPtr(out[i].TempLow, low, math.Min)
		out[i].TempHigh = pickPtr(out[i].TempHigh, high, math.Max)
	}
	return out
}

func pickPtr(a, b *float64, choose func(float64, float64) float64) *float64 {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	v := choose(*a, *b)
	return &v
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

// getUVData returns the UV forecast for the grid cell and whether this call
// just fetched it from FMI (as opposed to getting it from the in-process cache
// or from another request's fetch). Only a fresh fetch carries anything the
// persisted forecasts don't already have, so callers persist on fresh only. A
// failed or empty fetch is remembered for upstreamRetryAfter.
func (s *Service) getUVData(ctx context.Context, gridLat, gridLon float64, timezone string) ([]UVDataPoint, bool) {
	cacheKey := fmt.Sprintf("uv:%.2f,%.2f", gridLat, gridLon)
	if cached, ok := s.uvCache.Get(cacheKey); ok {
		return cached, false
	}

	points, leader, _ := s.uvFlight.Do(ctx, cacheKey, func(ctx context.Context) ([]UVDataPoint, error) {
		points, err := s.fmi.FetchUVForecast(ctx, gridLat, gridLon, startOfLocalDay(time.Now(), timezone))
		if err != nil {
			slog.Warn("UV forecast fetch failed", "err", err)
			points = nil
		} else if len(points) > 0 {
			slog.Info("fetched UV forecast from FMI", "lat", gridLat, "lon", gridLon, "points", len(points))
		}
		if len(points) == 0 {
			s.uvCache.SetTTL(cacheKey, nil, upstreamRetryAfter)
			return nil, nil
		}
		s.uvCache.Set(cacheKey, points)
		return points, nil
	})
	return points, leader && len(points) > 0
}

func placeLocation(timezone string) *time.Location {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func startOfLocalDay(now time.Time, timezone string) time.Time {
	local := now.In(placeLocation(timezone))
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
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

func applyUVToDaily(uvPoints []UVDataPoint, forecasts []DailyForecast, loc *time.Location) {
	type dailyUV struct {
		sum   float64
		count int
	}
	byDate := make(map[string]*dailyUV)
	for _, p := range uvPoints {
		date := p.Time.In(loc).Format("2006-01-02")
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

const dailyNormalsPeriod = "1991-2020"

// GetDailyClimateNormals serves the day-of-year normals computed from station
// history, from the station chosen by pickNormalsStations. A nil result with
// a nil error means no station within range has them.
func (s *Service) GetDailyClimateNormals(ctx context.Context, lat, lon float64, currentTemp *float64, now time.Time) (*DailyNormalsResult, error) {
	if !inCoverage(lat, lon) {
		return nil, nil
	}
	loc, err := time.LoadLocation(DefaultPlaceTimezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)

	candidates, err := s.store.DailyClimateNormalsCandidates(ctx, lat, lon, dailyNormalsPeriod, int(local.Month()), local.Day(), maxClimateNormalsDistanceKm)
	if err != nil {
		return nil, fmt.Errorf("daily climate normals candidates: %w", err)
	}
	startYear, endYear, err := parsePeriod(dailyNormalsPeriod)
	if err != nil {
		return nil, err
	}
	primary, hourlySource := pickNormalsStations(candidates, endYear-startYear+1)
	if primary == nil {
		return nil, nil
	}
	station, distKm := primary.Station, primary.DistanceKM

	normals, err := s.store.GetDailyClimateNormals(ctx, station.FMISID, dailyNormalsPeriod)
	if err != nil {
		return nil, fmt.Errorf("get daily climate normals: %w", err)
	}
	if len(normals) == 0 {
		return nil, nil
	}
	var hourlyStation *Station
	var hourlyDistKm float64
	if hourlySource != nil {
		hourlyNormals, err := s.store.GetDailyClimateNormals(ctx, hourlySource.FMISID, dailyNormalsPeriod)
		if err != nil {
			return nil, fmt.Errorf("get hourly daily climate normals: %w", err)
		}
		mergeHourlyNormals(normals, hourlyNormals)
		st := hourlySource.Station
		hourlyStation, hourlyDistKm = &st, hourlySource.DistanceKM
	}

	todayIdx := -1
	for i, n := range normals {
		if n.Month == int(local.Month()) && n.Day == local.Day() {
			todayIdx = i
			break
		}
	}
	if todayIdx < 0 {
		return nil, fmt.Errorf("no daily normal for %s", local.Format("01-02"))
	}
	today := normals[todayIdx]
	next := normals[(todayIdx+1)%len(normals)]

	res := &DailyNormalsResult{
		Station:            station,
		DistanceKM:         distKm,
		HourlyStation:      hourlyStation,
		HourlyDistanceKM:   hourlyDistKm,
		Period:             dailyNormalsPeriod,
		Today:              today,
		TempNowNormal:      HourlyNormalAt(today.TempHourly, next.TempHourly, now),
		FeelsLikeNowNormal: HourlyNormalAt(today.FeelsLikeHourly, next.FeelsLikeHourly, now),
		WindNowNormal:      HourlyNormalAt(today.WindHourly, next.WindHourly, now),
		HumidityNowNormal:  HourlyNormalAt(today.HumidityHourly, next.HumidityHourly, now),
		Daily:              normals,
	}
	if currentTemp != nil && res.TempNowNormal != nil {
		diff := *currentTemp - *res.TempNowNormal
		res.TempDiff = &diff
	}
	observed := s.observedPrecipitationThisMonth(ctx, lat, lon, now, loc)
	if observed != nil {
		res.Precipitation = precipitationToDate(normals, observed.Hourly, now, loc)
		st := observed.Station
		res.Precipitation.Station = &st
		res.Precipitation.StationDistanceKM = observed.DistanceKM
	} else {
		res.Precipitation = precipitationToDate(normals, nil, now, loc)
	}
	return res, nil
}

// observedPrecipitationThisMonth returns hourly precipitation since the first
// of the local month from the gauge nearest lat/lon, cached per ~1 km grid
// cell and hour. A fetch failure or missing gauge yields nil so the normals
// still serve; a failure is retried after upstreamRetryAfter. The fetch is
// shared between concurrent requests and outlives the one that started it.
func (s *Service) observedPrecipitationThisMonth(ctx context.Context, lat, lon float64, now time.Time, loc *time.Location) *PrecipitationObservations {
	if s.precipHistory == nil {
		return nil
	}
	local := now.In(loc)
	gridLat, gridLon := snapToGrid(lat, lon)
	key := fmt.Sprintf("%.2f:%.2f:%s", gridLat, gridLon, local.Format("2006-01-02T15"))
	if cached, ok := s.precipToDateCache.Get(key); ok {
		return cached
	}
	monthStart := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
	observed, _, err := s.precipToDateFlight.Do(ctx, key, func(ctx context.Context) (*PrecipitationObservations, error) {
		observed, err := s.precipHistory.FetchHourlyPrecipitationNear(ctx, lat, lon, monthStart, now)
		if errors.Is(err, ErrNoStation) {
			s.precipToDateCache.Set(key, nil)
			return nil, nil
		}
		if err != nil {
			slog.Warn("observed precipitation this month", "lat", lat, "lon", lon, "err", err)
			s.precipToDateCache.SetTTL(key, nil, upstreamRetryAfter)
			return nil, nil
		}
		s.precipToDateCache.Set(key, &observed)
		return &observed, nil
	})
	if err != nil {
		return nil
	}
	return observed
}

func (s *Service) GetClimateNormals(ctx context.Context, lat, lon float64, currentTemp *float64) (*Station, float64, []ClimateNormal, InterpolatedNormal, error) {
	if !inCoverage(lat, lon) {
		return nil, 0, nil, InterpolatedNormal{}, nil
	}
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
