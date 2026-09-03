package weather

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"
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

// ErrNoStation is returned by store lookups that found no matching station.
var ErrNoStation = errors.New("no station found")

// ErrForecastGridUnavailable is returned for a future-time temperature request
// whose hourly GRIB frame isn't in the warmed cache.
var ErrForecastGridUnavailable = errors.New("forecast grid not available")

type WeatherStore interface {
	NearestStation(ctx context.Context, lat, lon float64) (Station, float64, error)
	LatestObservation(ctx context.Context, fmisid int) (Observation, error)
	ObservedTemperatureRange(ctx context.Context, fmisid int, from, to time.Time) (low, high *float64, err error)
	GetLatestTemperatureSamplesInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, limit int) ([]TemperatureSample, error)
	GetObservationSamplesAtTimeInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time, limit int) ([]TemperatureSample, error)
	GetForecasts(ctx context.Context, gridLat, gridLon float64) ([]DailyForecast, error)
	UpsertForecasts(ctx context.Context, forecasts []DailyForecast) error
	GetHourlyForecasts(ctx context.Context, gridLat, gridLon float64, limit int) ([]HourlyForecast, error)
	UpsertHourlyForecasts(ctx context.Context, gridLat, gridLon float64, hourly []HourlyForecast) error
	UpsertClimateNormals(ctx context.Context, normals []ClimateNormal) error
	GetClimateNormals(ctx context.Context, fmisid int, period string) ([]ClimateNormal, error)
	NearestStationWithClimateNormals(ctx context.Context, lat, lon float64, period string) (Station, float64, error)
	NearestStationWithDailyClimateNormals(ctx context.Context, lat, lon float64, period string) (Station, float64, error)
	GetDailyClimateNormals(ctx context.Context, fmisid int, period string) ([]DailyClimateNormal, error)
	UpsertDailyClimateNormals(ctx context.Context, normals []DailyClimateNormal) error
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
		gribGridCache:    NewCache[*FieldGrid](0),
		gribPrecipCache:  NewCache[*PrecipitationForecastGrid](0),
		radarPrecipCache: NewCache[*PrecipitationForecastGrid](radarGridCacheTTL),
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

	forecast = widenWithObservedRange(forecast, time.Now().UTC(), func(from, to time.Time) (*float64, *float64, error) {
		return s.store.ObservedTemperatureRange(ctx, station.FMISID, from, to)
	})

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
	slog.Info("warmed grib grids",
		"precipitation_frames", precip,
		"temperature_frames", temp,
		"duration", time.Since(start),
	)
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
func widenWithObservedRange(forecast []DailyForecast, now time.Time, observed func(from, to time.Time) (low, high *float64, err error)) []DailyForecast {
	out := slices.Clone(forecast)
	for i := range out {
		start := out[i].Date
		if !start.Before(now) {
			continue
		}
		end := start.Add(24 * time.Hour)
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

const dailyNormalsPeriod = "1991-2020"

// GetDailyClimateNormals serves the day-of-year normals computed from station
// history. A nil result with a nil error means no station within range has
// them.
func (s *Service) GetDailyClimateNormals(ctx context.Context, lat, lon float64, currentTemp *float64, now time.Time) (*DailyNormalsResult, error) {
	station, distKm, err := s.store.NearestStationWithDailyClimateNormals(ctx, lat, lon, dailyNormalsPeriod)
	if errors.Is(err, ErrNoStation) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("nearest station with daily climate normals: %w", err)
	}
	if distKm > maxClimateNormalsDistanceKm {
		return nil, nil
	}

	normals, err := s.store.GetDailyClimateNormals(ctx, station.FMISID, dailyNormalsPeriod)
	if err != nil {
		return nil, fmt.Errorf("get daily climate normals: %w", err)
	}
	if len(normals) == 0 {
		return nil, nil
	}

	loc, err := time.LoadLocation(DefaultPlaceTimezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
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
	var next []float64
	if todayIdx+1 < len(normals) {
		next = normals[todayIdx+1].TempHourly
	} else {
		next = normals[0].TempHourly
	}

	res := &DailyNormalsResult{
		Station:       station,
		DistanceKM:    distKm,
		Period:        dailyNormalsPeriod,
		Today:         today,
		TempNowNormal: HourlyNormalAt(today.TempHourly, next, now),
		Daily:         normals,
	}
	if currentTemp != nil && res.TempNowNormal != nil {
		diff := *currentTemp - *res.TempNowNormal
		res.TempDiff = &diff
	}
	return res, nil
}

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
