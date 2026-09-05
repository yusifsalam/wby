package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"wby/internal/weather"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) UpsertStations(ctx context.Context, stations []weather.Station) error {
	batch := &pgx.Batch{}
	for _, st := range stations {
		batch.Queue(
			`INSERT INTO stations (fmisid, name, geom, wmo_code)
			 VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography, $5)
			 ON CONFLICT (fmisid) DO UPDATE SET name = $2, geom = ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography, wmo_code = $5`,
			st.FMISID, st.Name, st.Lon, st.Lat, st.WMOCode,
		)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range stations {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upsert station: %w", err)
		}
	}
	return nil
}

func (s *Store) NearestStation(ctx context.Context, lat, lon float64) (weather.Station, float64, error) {
	var st weather.Station
	var distMeters float64
	err := s.pool.QueryRow(ctx,
		`SELECT fmisid, name, ST_Y(geom::geometry), ST_X(geom::geometry), wmo_code,
		        ST_Distance(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)
		 FROM stations
		 ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
		 LIMIT 1`,
		lon, lat,
	).Scan(&st.FMISID, &st.Name, &st.Lat, &st.Lon, &st.WMOCode, &distMeters)
	if err != nil {
		return st, 0, fmt.Errorf("nearest station: %w", err)
	}
	return st, distMeters / 1000.0, nil
}

// NearestStations returns up to limit stations within radiusKm of the point,
// nearest first.
func (s *Store) NearestStations(ctx context.Context, lat, lon, radiusKm float64, limit int) ([]weather.StationDistance, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT fmisid, name, ST_Y(geom::geometry), ST_X(geom::geometry), wmo_code,
		        ST_Distance(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)
		 FROM stations
		 WHERE ST_DWithin(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		 ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
		 LIMIT $4`,
		lon, lat, radiusKm*1000, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("nearest stations: %w", err)
	}
	defer rows.Close()

	var out []weather.StationDistance
	for rows.Next() {
		var sd weather.StationDistance
		var distMeters float64
		if err := rows.Scan(&sd.FMISID, &sd.Name, &sd.Lat, &sd.Lon, &sd.WMOCode, &distMeters); err != nil {
			return nil, fmt.Errorf("scan nearest station: %w", err)
		}
		sd.DistanceKM = distMeters / 1000.0
		out = append(out, sd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nearest stations: %w", err)
	}
	return out, nil
}

func (s *Store) NearestStationWithClimateNormals(ctx context.Context, lat, lon float64, period string) (weather.Station, float64, error) {
	var st weather.Station
	var distMeters float64
	err := s.pool.QueryRow(ctx,
		`SELECT s.fmisid, s.name, ST_Y(s.geom::geometry), ST_X(s.geom::geometry), s.wmo_code,
		        ST_Distance(s.geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)
		 FROM stations s
		 WHERE EXISTS (SELECT 1 FROM climate_normals cn WHERE cn.fmisid = s.fmisid AND cn.period = $3)
		 ORDER BY s.geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
		 LIMIT 1`,
		lon, lat, period,
	).Scan(&st.FMISID, &st.Name, &st.Lat, &st.Lon, &st.WMOCode, &distMeters)
	if err != nil {
		return st, 0, fmt.Errorf("nearest station with climate normals: %w", err)
	}
	return st, distMeters / 1000.0, nil
}

func (s *Store) UpsertObservations(ctx context.Context, observations []weather.Observation) error {
	batch := &pgx.Batch{}
	for _, o := range observations {
		extra := encodeNumericExtras(o.ExtraNumericParams)
		batch.Queue(
			`INSERT INTO observations (
				fmisid, observed_at, temperature, wind_speed, wind_gust, wind_dir, humidity, dew_point,
				pressure, precip_1h, precip_intensity, snow_depth, visibility, total_cloud_cover, weather_code, extra
			)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			 ON CONFLICT (fmisid, observed_at) DO UPDATE SET
			   temperature = $3, wind_speed = $4, wind_gust = $5, wind_dir = $6, humidity = $7, dew_point = $8,
			   pressure = $9, precip_1h = $10, precip_intensity = $11, snow_depth = $12, visibility = $13,
			   total_cloud_cover = $14, weather_code = $15, extra = $16`,
			o.FMISID, o.ObservedAt, o.Temperature, o.WindSpeed, o.WindGust, o.WindDir, o.Humidity, o.DewPoint,
			o.Pressure, o.Precip1h, o.PrecipIntensity, o.SnowDepth, o.Visibility, o.TotalCloudCover, o.WeatherCode, extra,
		)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range observations {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upsert observation: %w", err)
		}
	}
	return nil
}

// latestObservationWindow bounds how far back LatestObservation looks for a
// value of each parameter. FMI reports some parameters (precip_1h) only on
// the hour and occasionally skips others (wind) for a 10-minute slot, so a
// single row rarely carries every field. 70 minutes keeps an hourly value
// alive until the next one arrives.
const latestObservationWindow = "70 minutes"

// LatestObservation composes current conditions for a station from the most
// recent non-null value of each parameter within latestObservationWindow of
// the station's newest row. ObservedAt is that newest row's timestamp.
func (s *Store) LatestObservation(ctx context.Context, fmisid int) (weather.Observation, error) {
	all, err := s.LatestObservations(ctx, []int{fmisid})
	if err != nil {
		return weather.Observation{}, err
	}
	o, ok := all[fmisid]
	if !ok {
		return weather.Observation{}, fmt.Errorf("latest observation: %w", pgx.ErrNoRows)
	}
	return o, nil
}

// LatestObservations is LatestObservation for several stations at once,
// keyed by FMISID. Stations without any observation are absent from the map.
func (s *Store) LatestObservations(ctx context.Context, fmisids []int) (map[int]weather.Observation, error) {
	rows, err := s.pool.Query(ctx,
		`WITH latest AS (
		   SELECT fmisid, max(observed_at) AS at FROM observations WHERE fmisid = ANY($1) GROUP BY fmisid
		 ), recent AS (
		   SELECT o.* FROM observations o JOIN latest l ON l.fmisid = o.fmisid
		   WHERE o.observed_at > l.at - INTERVAL '`+latestObservationWindow+`'
		 )
		 SELECT fmisid, max(observed_at),
		        (array_agg(temperature ORDER BY observed_at DESC) FILTER (WHERE temperature IS NOT NULL))[1],
		        (array_agg(wind_speed ORDER BY observed_at DESC) FILTER (WHERE wind_speed IS NOT NULL))[1],
		        (array_agg(wind_gust ORDER BY observed_at DESC) FILTER (WHERE wind_gust IS NOT NULL))[1],
		        (array_agg(wind_dir ORDER BY observed_at DESC) FILTER (WHERE wind_dir IS NOT NULL))[1],
		        (array_agg(humidity ORDER BY observed_at DESC) FILTER (WHERE humidity IS NOT NULL))[1],
		        (array_agg(dew_point ORDER BY observed_at DESC) FILTER (WHERE dew_point IS NOT NULL))[1],
		        (array_agg(pressure ORDER BY observed_at DESC) FILTER (WHERE pressure IS NOT NULL))[1],
		        (array_agg(precip_1h ORDER BY observed_at DESC) FILTER (WHERE precip_1h IS NOT NULL))[1],
		        (array_agg(precip_intensity ORDER BY observed_at DESC) FILTER (WHERE precip_intensity IS NOT NULL))[1],
		        (array_agg(snow_depth ORDER BY observed_at DESC) FILTER (WHERE snow_depth IS NOT NULL))[1],
		        (array_agg(visibility ORDER BY observed_at DESC) FILTER (WHERE visibility IS NOT NULL))[1],
		        (array_agg(total_cloud_cover ORDER BY observed_at DESC) FILTER (WHERE total_cloud_cover IS NOT NULL))[1],
		        (array_agg(weather_code ORDER BY observed_at DESC) FILTER (WHERE weather_code IS NOT NULL))[1],
		        (array_agg(extra ORDER BY observed_at DESC) FILTER (WHERE extra IS NOT NULL))[1]
		 FROM recent
		 GROUP BY fmisid`,
		fmisids,
	)
	if err != nil {
		return nil, fmt.Errorf("latest observations: %w", err)
	}
	defer rows.Close()

	out := make(map[int]weather.Observation, len(fmisids))
	for rows.Next() {
		var o weather.Observation
		var extraRaw []byte
		if err := rows.Scan(
			&o.FMISID, &o.ObservedAt, &o.Temperature, &o.WindSpeed, &o.WindGust, &o.WindDir, &o.Humidity, &o.DewPoint,
			&o.Pressure, &o.Precip1h, &o.PrecipIntensity, &o.SnowDepth, &o.Visibility, &o.TotalCloudCover, &o.WeatherCode, &extraRaw,
		); err != nil {
			return nil, fmt.Errorf("scan latest observation: %w", err)
		}
		o.ExtraNumericParams = decodeNumericExtras(extraRaw)
		out[o.FMISID] = o
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("latest observations: %w", err)
	}
	return out, nil
}

func (s *Store) ObservedTemperatureRange(ctx context.Context, fmisid int, from, to time.Time) (low, high *float64, err error) {
	err = s.pool.QueryRow(ctx,
		`SELECT min(temperature), max(temperature) FROM observations
		 WHERE fmisid = $1 AND observed_at >= $2 AND observed_at < $3`,
		fmisid, from, to,
	).Scan(&low, &high)
	if err != nil {
		return nil, nil, fmt.Errorf("observed temperature range: %w", err)
	}
	return low, high, nil
}

func (s *Store) GetLatestTemperatureSamplesInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, limit int) ([]weather.TemperatureSample, error) {
	if limit <= 0 {
		limit = 300
	}

	rows, err := s.pool.Query(ctx,
		`SELECT ST_Y(s.geom::geometry) AS lat,
		        ST_X(s.geom::geometry) AS lon,
		        o.temperature,
		        o.extra,
		        o.observed_at
		 FROM stations s
		 JOIN LATERAL (
		    SELECT temperature, extra, observed_at
		    FROM observations o
		    WHERE o.fmisid = s.fmisid
		      AND o.observed_at > NOW() - INTERVAL '2 hours'
		    ORDER BY observed_at DESC
		    LIMIT 1
		 ) o ON true
		 WHERE ST_X(s.geom::geometry) BETWEEN $1 AND $2
		   AND ST_Y(s.geom::geometry) BETWEEN $3 AND $4
		 ORDER BY o.observed_at DESC
		 LIMIT $5`,
		minLon, maxLon, minLat, maxLat, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query temperature samples: %w", err)
	}
	defer rows.Close()

	result := make([]weather.TemperatureSample, 0, limit)
	for rows.Next() {
		var (
			lat      float64
			lon      float64
			temp     *float64
			extraRaw []byte
			at       time.Time
		)
		if err := rows.Scan(&lat, &lon, &temp, &extraRaw, &at); err != nil {
			return nil, fmt.Errorf("scan temperature sample: %w", err)
		}

		resolved := temp
		if resolved == nil && len(extraRaw) > 0 {
			extra := decodeNumericExtras(extraRaw)
			if t2m, ok := extra["t2m"]; ok {
				resolved = &t2m
			}
		}
		if resolved == nil {
			continue
		}

		result = append(result, weather.TemperatureSample{
			Lat:         lat,
			Lon:         lon,
			Temperature: *resolved,
			ObservedAt:  at,
		})
	}
	return result, nil
}

func (s *Store) GetObservationSamplesAtTimeInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time, limit int) ([]weather.TemperatureSample, error) {
	if limit <= 0 {
		limit = 300
	}

	rows, err := s.pool.Query(ctx,
		`SELECT ST_Y(s.geom::geometry) AS lat,
		        ST_X(s.geom::geometry) AS lon,
		        o.temperature,
		        o.extra,
		        o.observed_at
		 FROM stations s
		 JOIN LATERAL (
		    SELECT temperature, extra, observed_at
		    FROM observations o
		    WHERE o.fmisid = s.fmisid
		      AND o.observed_at BETWEEN ($5::timestamptz - INTERVAL '30 minutes') AND ($5::timestamptz + INTERVAL '30 minutes')
		    ORDER BY ABS(EXTRACT(EPOCH FROM (o.observed_at - $5::timestamptz)))
		    LIMIT 1
		 ) o ON true
		 WHERE ST_X(s.geom::geometry) BETWEEN $1 AND $2
		   AND ST_Y(s.geom::geometry) BETWEEN $3 AND $4
		 ORDER BY o.observed_at DESC
		 LIMIT $6`,
		minLon, maxLon, minLat, maxLat, at, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query observation samples at time: %w", err)
	}
	defer rows.Close()

	result := make([]weather.TemperatureSample, 0, limit)
	for rows.Next() {
		var (
			lat      float64
			lon      float64
			temp     *float64
			extraRaw []byte
			obsAt    time.Time
		)
		if err := rows.Scan(&lat, &lon, &temp, &extraRaw, &obsAt); err != nil {
			return nil, fmt.Errorf("scan observation sample: %w", err)
		}

		resolved := temp
		if resolved == nil && len(extraRaw) > 0 {
			extra := decodeNumericExtras(extraRaw)
			if t2m, ok := extra["t2m"]; ok {
				resolved = &t2m
			}
		}
		if resolved == nil {
			continue
		}

		result = append(result, weather.TemperatureSample{
			Lat:         lat,
			Lon:         lon,
			Temperature: *resolved,
			ObservedAt:  obsAt,
		})
	}
	return result, nil
}

func encodeNumericExtras(params map[string]float64) []byte {
	if len(params) == 0 {
		return nil
	}
	b, err := json.Marshal(params)
	if err != nil {
		return nil
	}
	return b
}

func decodeNumericExtras(raw []byte) map[string]float64 {
	if len(raw) == 0 {
		return nil
	}
	var result map[string]float64
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	return result
}

func (s *Store) UpsertForecasts(ctx context.Context, forecasts []weather.DailyForecast) error {
	batch := &pgx.Batch{}
	for _, f := range forecasts {
		batch.Queue(
			`INSERT INTO forecasts (
				grid_lat, grid_lon, forecast_for, fetched_at, temp_high, temp_low,
				temp_avg, wind_speed, wind_direction, humidity_avg, precip_mm, precipitation_1h_sum, symbol,
				dew_point_avg, fog_intensity_avg, frost_probability_avg, severe_frost_probability_avg, geop_height_avg, pressure_avg,
				high_cloud_cover_avg, low_cloud_cover_avg, medium_cloud_cover_avg, middle_and_low_cloud_cover_avg, total_cloud_cover_avg,
				hourly_maximum_gust_max, hourly_maximum_wind_speed_max, pop_avg, probability_thunderstorm_avg,
				potential_precipitation_form_mode, potential_precipitation_type_mode, precipitation_form_mode, precipitation_type_mode,
				radiation_global_avg, radiation_lw_avg, weather_number_mode, weather_symbol3_mode, wind_ums_avg, wind_vms_avg, wind_vector_ms_avg,
				uv_index_avg
			)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40)
			 ON CONFLICT (grid_lat, grid_lon, forecast_for) DO UPDATE SET
			   fetched_at = $4, temp_high = $5, temp_low = $6, temp_avg = $7, wind_speed = $8, wind_direction = $9,
			   humidity_avg = $10, precip_mm = $11, precipitation_1h_sum = $12, symbol = $13, dew_point_avg = $14,
			   fog_intensity_avg = $15, frost_probability_avg = $16, severe_frost_probability_avg = $17, geop_height_avg = $18, pressure_avg = $19,
			   high_cloud_cover_avg = $20, low_cloud_cover_avg = $21, medium_cloud_cover_avg = $22, middle_and_low_cloud_cover_avg = $23,
			   total_cloud_cover_avg = $24, hourly_maximum_gust_max = $25, hourly_maximum_wind_speed_max = $26, pop_avg = $27,
			   probability_thunderstorm_avg = $28, potential_precipitation_form_mode = $29, potential_precipitation_type_mode = $30,
			   precipitation_form_mode = $31, precipitation_type_mode = $32, radiation_global_avg = $33, radiation_lw_avg = $34,
			   weather_number_mode = $35, weather_symbol3_mode = $36, wind_ums_avg = $37, wind_vms_avg = $38, wind_vector_ms_avg = $39,
			   uv_index_avg = $40`,
			f.GridLat, f.GridLon, f.Date, f.FetchedAt, f.TempHigh, f.TempLow,
			f.TempAvg, f.WindSpeed, f.WindDir, f.HumidityAvg, f.PrecipMM, f.Precip1hSum, f.Symbol,
			f.DewPointAvg, f.FogIntensityAvg, f.FrostProbabilityAvg, f.SevereFrostProbabilityAvg, f.GeopHeightAvg, f.PressureAvg,
			f.HighCloudCoverAvg, f.LowCloudCoverAvg, f.MediumCloudCoverAvg, f.MiddleAndLowCloudCoverAvg, f.TotalCloudCoverAvg,
			f.HourlyMaximumGustMax, f.HourlyMaximumWindSpeedMax, f.PoPAvg, f.ProbabilityThunderstormAvg,
			f.PotentialPrecipitationFormMode, f.PotentialPrecipitationTypeMode, f.PrecipitationFormMode, f.PrecipitationTypeMode,
			f.RadiationGlobalAvg, f.RadiationLWAvg, f.WeatherNumberMode, f.WeatherSymbol3Mode, f.WindUMSAvg, f.WindVMSAvg, f.WindVectorMSAvg,
			f.UVIndexAvg,
		)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range forecasts {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upsert forecast: %w", err)
		}
	}
	return nil
}

func (s *Store) GetForecasts(ctx context.Context, gridLat, gridLon float64) ([]weather.DailyForecast, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT grid_lat, grid_lon, forecast_for, fetched_at, temp_high, temp_low,
		        temp_avg, wind_speed, wind_direction, humidity_avg, precip_mm, precipitation_1h_sum, symbol,
		        dew_point_avg, fog_intensity_avg, frost_probability_avg, severe_frost_probability_avg, geop_height_avg, pressure_avg,
		        high_cloud_cover_avg, low_cloud_cover_avg, medium_cloud_cover_avg, middle_and_low_cloud_cover_avg, total_cloud_cover_avg,
		        hourly_maximum_gust_max, hourly_maximum_wind_speed_max, pop_avg, probability_thunderstorm_avg,
		        potential_precipitation_form_mode, potential_precipitation_type_mode, precipitation_form_mode, precipitation_type_mode,
		        radiation_global_avg, radiation_lw_avg, weather_number_mode, weather_symbol3_mode, wind_ums_avg, wind_vms_avg, wind_vector_ms_avg,
		        uv_index_avg
		 FROM forecasts
		 WHERE grid_lat = $1 AND grid_lon = $2 AND forecast_for >= CURRENT_DATE
		 ORDER BY forecast_for
		 LIMIT 11`,
		gridLat, gridLon,
	)
	if err != nil {
		return nil, fmt.Errorf("get forecasts: %w", err)
	}
	defer rows.Close()

	var result []weather.DailyForecast
	for rows.Next() {
		var f weather.DailyForecast
		if err := rows.Scan(
			&f.GridLat, &f.GridLon, &f.Date, &f.FetchedAt, &f.TempHigh, &f.TempLow,
			&f.TempAvg, &f.WindSpeed, &f.WindDir, &f.HumidityAvg, &f.PrecipMM, &f.Precip1hSum, &f.Symbol,
			&f.DewPointAvg, &f.FogIntensityAvg, &f.FrostProbabilityAvg, &f.SevereFrostProbabilityAvg, &f.GeopHeightAvg, &f.PressureAvg,
			&f.HighCloudCoverAvg, &f.LowCloudCoverAvg, &f.MediumCloudCoverAvg, &f.MiddleAndLowCloudCoverAvg, &f.TotalCloudCoverAvg,
			&f.HourlyMaximumGustMax, &f.HourlyMaximumWindSpeedMax, &f.PoPAvg, &f.ProbabilityThunderstormAvg,
			&f.PotentialPrecipitationFormMode, &f.PotentialPrecipitationTypeMode, &f.PrecipitationFormMode, &f.PrecipitationTypeMode,
			&f.RadiationGlobalAvg, &f.RadiationLWAvg, &f.WeatherNumberMode, &f.WeatherSymbol3Mode, &f.WindUMSAvg, &f.WindVMSAvg, &f.WindVectorMSAvg,
			&f.UVIndexAvg,
		); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, nil
}

func (s *Store) UpsertHourlyForecasts(ctx context.Context, gridLat, gridLon float64, hourly []weather.HourlyForecast) error {
	batch := &pgx.Batch{}
	now := time.Now()
	for _, h := range hourly {
		fetchedAt := h.FetchedAt
		if fetchedAt.IsZero() {
			fetchedAt = now
		}
		batch.Queue(
			`INSERT INTO hourly_forecasts (
				grid_lat, grid_lon, forecast_time, fetched_at,
				temperature, wind_speed, wind_direction, humidity, precipitation_1h, symbol, uv_cumulated,
				wind_gust, pressure, cloud_cover, pop, feels_like
			)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			 ON CONFLICT (grid_lat, grid_lon, forecast_time) DO UPDATE SET
			   fetched_at = $4, temperature = $5, wind_speed = $6, wind_direction = $7,
			   humidity = $8, precipitation_1h = $9, symbol = $10, uv_cumulated = $11,
			   wind_gust = $12, pressure = $13, cloud_cover = $14, pop = $15, feels_like = $16`,
			gridLat, gridLon, h.Time, fetchedAt,
			h.Temperature, h.WindSpeed, h.WindDir, h.Humidity, h.Precip1h, h.Symbol, h.UVCumulated,
			h.WindGust, h.Pressure, h.CloudCover, h.PoP, h.FeelsLike,
		)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range hourly {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upsert hourly forecast: %w", err)
		}
	}
	_, _ = s.pool.Exec(ctx,
		`DELETE FROM hourly_forecasts
		 WHERE forecast_time < (NOW() - INTERVAL '3 days')`,
	)
	return nil
}

func (s *Store) GetHourlyForecasts(ctx context.Context, gridLat, gridLon float64, limit int) ([]weather.HourlyForecast, error) {
	if limit <= 0 {
		limit = 12
	}
	rows, err := s.pool.Query(ctx,
		`SELECT forecast_time, fetched_at, temperature, wind_speed, wind_direction, humidity, precipitation_1h, symbol, uv_cumulated,
		        wind_gust, pressure, cloud_cover, pop, feels_like
		 FROM hourly_forecasts
		 WHERE grid_lat = $1 AND grid_lon = $2 AND forecast_time >= date_trunc('hour', NOW())
		 ORDER BY forecast_time
		 LIMIT $3`,
		gridLat, gridLon, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get hourly forecasts: %w", err)
	}
	defer rows.Close()

	var result []weather.HourlyForecast
	for rows.Next() {
		var h weather.HourlyForecast
		if err := rows.Scan(
			&h.Time, &h.FetchedAt, &h.Temperature, &h.WindSpeed, &h.WindDir, &h.Humidity, &h.Precip1h, &h.Symbol, &h.UVCumulated,
			&h.WindGust, &h.Pressure, &h.CloudCover, &h.PoP, &h.FeelsLike,
		); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, nil
}

func (s *Store) AllStationFMISIDs(ctx context.Context) ([]int, error) {
	rows, err := s.pool.Query(ctx, "SELECT fmisid FROM stations ORDER BY fmisid")
	if err != nil {
		return nil, fmt.Errorf("list station fmisids: %w", err)
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan fmisid: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) UpsertClimateNormals(ctx context.Context, normals []weather.ClimateNormal) error {
	batch := &pgx.Batch{}
	for _, n := range normals {
		batch.Queue(`
			INSERT INTO climate_normals (fmisid, month, period, temp_avg, temp_high, temp_low, precip_mm)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (fmisid, month, period) DO UPDATE SET
				temp_avg = EXCLUDED.temp_avg,
				temp_high = EXCLUDED.temp_high,
				temp_low = EXCLUDED.temp_low,
				precip_mm = EXCLUDED.precip_mm`,
			n.FMISID, n.Month, n.Period, n.TempAvg, n.TempHigh, n.TempLow, n.PrecipMm)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range normals {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upsert climate normals: %w", err)
		}
	}
	return nil
}

func (s *Store) GetLeaderboard(ctx context.Context, lat, lon float64, timeframe string) ([]weather.LeaderboardEntry, error) {
	interval := timeframeToInterval(timeframe)

	var query string
	if timeframe == "now" {
		// Latest observation per station, then find extremes.
		query = `
		WITH latest AS (
			SELECT DISTINCT ON (s.fmisid)
				s.fmisid, s.name,
				ST_Y(s.geom::geometry) AS lat,
				ST_X(s.geom::geometry) AS lon,
				ST_Distance(s.geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) / 1000.0 AS distance_km,
				o.temperature, o.wind_speed, o.observed_at
			FROM stations s
			JOIN observations o ON o.fmisid = s.fmisid
			WHERE o.observed_at >= NOW() - INTERVAL '` + interval + `'
			ORDER BY s.fmisid, o.observed_at DESC
		)
		(SELECT 'coldest' AS stat_type, name, lat, lon, temperature AS value, distance_km, observed_at
		 FROM latest WHERE temperature IS NOT NULL ORDER BY temperature ASC LIMIT 1)
		UNION ALL
		(SELECT 'warmest', name, lat, lon, temperature, distance_km, observed_at
		 FROM latest WHERE temperature IS NOT NULL ORDER BY temperature DESC LIMIT 1)
		UNION ALL
		(SELECT 'windiest', name, lat, lon, wind_speed, distance_km, observed_at
		 FROM latest WHERE wind_speed IS NOT NULL ORDER BY wind_speed DESC LIMIT 1)`
	} else {
		// Scan all observations in the window for the actual extremes.
		query = `
		(SELECT 'coldest' AS stat_type, s.name,
		        ST_Y(s.geom::geometry) AS lat, ST_X(s.geom::geometry) AS lon,
		        o.temperature AS value,
		        ST_Distance(s.geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) / 1000.0 AS distance_km,
		        o.observed_at
		 FROM observations o
		 JOIN stations s ON s.fmisid = o.fmisid
		 WHERE o.observed_at >= NOW() - INTERVAL '` + interval + `'
		   AND o.temperature IS NOT NULL
		 ORDER BY o.temperature ASC LIMIT 1)
		UNION ALL
		(SELECT 'warmest', s.name,
		        ST_Y(s.geom::geometry), ST_X(s.geom::geometry),
		        o.temperature,
		        ST_Distance(s.geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) / 1000.0,
		        o.observed_at
		 FROM observations o
		 JOIN stations s ON s.fmisid = o.fmisid
		 WHERE o.observed_at >= NOW() - INTERVAL '` + interval + `'
		   AND o.temperature IS NOT NULL
		 ORDER BY o.temperature DESC LIMIT 1)
		UNION ALL
		(SELECT 'windiest', s.name,
		        ST_Y(s.geom::geometry), ST_X(s.geom::geometry),
		        o.wind_speed,
		        ST_Distance(s.geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) / 1000.0,
		        o.observed_at
		 FROM observations o
		 JOIN stations s ON s.fmisid = o.fmisid
		 WHERE o.observed_at >= NOW() - INTERVAL '` + interval + `'
		   AND o.wind_speed IS NOT NULL
		 ORDER BY o.wind_speed DESC LIMIT 1)`
	}

	rows, err := s.pool.Query(ctx, query, lon, lat)
	if err != nil {
		return nil, fmt.Errorf("get leaderboard: %w", err)
	}
	defer rows.Close()

	var entries []weather.LeaderboardEntry
	for rows.Next() {
		var e weather.LeaderboardEntry
		if err := rows.Scan(&e.StatType, &e.StationName, &e.Lat, &e.Lon, &e.Value, &e.DistanceKM, &e.ObservedAt); err != nil {
			return nil, fmt.Errorf("scan leaderboard entry: %w", err)
		}
		switch e.StatType {
		case "coldest", "warmest":
			e.Unit = "°C"
		case "windiest":
			e.Unit = "m/s"
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func timeframeToInterval(tf string) string {
	switch tf {
	case "1h":
		return "1 hour"
	case "24h":
		return "24 hours"
	case "3d":
		return "3 days"
	case "7d":
		return "7 days"
	default:
		return "2 hours"
	}
}

// DailyClimateNormalsCandidates lists the stations within radiusKm that have
// a daily normal row for the calendar day, nearest first.
func (s *Store) DailyClimateNormalsCandidates(ctx context.Context, lat, lon float64, period string, month, day int, radiusKm float64) ([]weather.DailyNormalsCandidate, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT s.fmisid, s.name, ST_Y(s.geom::geometry), ST_X(s.geom::geometry), s.wmo_code,
		        ST_Distance(s.geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography),
		        COALESCE(n.daily_years, 0), COALESCE(n.hourly_years, 0),
		        n.temp_avg IS NOT NULL, n.temp_hourly IS NOT NULL
		 FROM stations s
		 JOIN daily_climate_normals n ON n.fmisid = s.fmisid
		 WHERE n.period = $3 AND n.month = $4 AND n.day = $5
		   AND ST_DWithin(s.geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $6)
		 ORDER BY s.geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography`,
		lon, lat, period, month, day, radiusKm*1000,
	)
	if err != nil {
		return nil, fmt.Errorf("daily climate normals candidates: %w", err)
	}
	defer rows.Close()

	var out []weather.DailyNormalsCandidate
	for rows.Next() {
		var c weather.DailyNormalsCandidate
		var distMeters float64
		if err := rows.Scan(&c.FMISID, &c.Name, &c.Lat, &c.Lon, &c.WMOCode, &distMeters,
			&c.DailyYears, &c.HourlyYears, &c.HasTemp, &c.HasHourly); err != nil {
			return nil, fmt.Errorf("scan daily climate normals candidate: %w", err)
		}
		c.DistanceKM = distMeters / 1000
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("daily climate normals candidates: %w", err)
	}
	return out, nil
}

func (s *Store) UpsertDailyClimateNormals(ctx context.Context, normals []weather.DailyClimateNormal) error {
	batch := &pgx.Batch{}
	for _, n := range normals {
		batch.Queue(`
			INSERT INTO daily_climate_normals (
				fmisid, period, month, day,
				temp_avg, temp_high, temp_low,
				feels_like_avg, feels_like_high, feels_like_low,
				wind_avg, wind_gust, humidity_avg,
				precip_mm, precip_days_pct, snow_cm,
				temp_hourly, feels_like_hourly, wind_hourly, humidity_hourly,
				temp_hourly_p10, temp_hourly_p90, daily_years, hourly_years)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
			ON CONFLICT (fmisid, period, month, day) DO UPDATE SET
				temp_avg = EXCLUDED.temp_avg,
				temp_high = EXCLUDED.temp_high,
				temp_low = EXCLUDED.temp_low,
				feels_like_avg = EXCLUDED.feels_like_avg,
				feels_like_high = EXCLUDED.feels_like_high,
				feels_like_low = EXCLUDED.feels_like_low,
				wind_avg = EXCLUDED.wind_avg,
				wind_gust = EXCLUDED.wind_gust,
				humidity_avg = EXCLUDED.humidity_avg,
				precip_mm = EXCLUDED.precip_mm,
				precip_days_pct = EXCLUDED.precip_days_pct,
				snow_cm = EXCLUDED.snow_cm,
				temp_hourly = EXCLUDED.temp_hourly,
				feels_like_hourly = EXCLUDED.feels_like_hourly,
				wind_hourly = EXCLUDED.wind_hourly,
				humidity_hourly = EXCLUDED.humidity_hourly,
				temp_hourly_p10 = EXCLUDED.temp_hourly_p10,
				temp_hourly_p90 = EXCLUDED.temp_hourly_p90,
				daily_years = EXCLUDED.daily_years,
				hourly_years = EXCLUDED.hourly_years`,
			n.FMISID, n.Period, n.Month, n.Day,
			n.TempAvg, n.TempHigh, n.TempLow,
			n.FeelsLikeAvg, n.FeelsLikeHigh, n.FeelsLikeLow,
			n.WindAvg, n.WindGust, n.HumidityAvg,
			n.PrecipMm, n.PrecipDaysPct, n.SnowCm,
			n.TempHourly, n.FeelsLikeHourly, n.WindHourly, n.HumidityHourly,
			n.TempHourlyP10, n.TempHourlyP90, n.DailyYears, n.HourlyYears)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range normals {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("upsert daily climate normals: %w", err)
		}
	}
	return nil
}

func (s *Store) GetDailyClimateNormals(ctx context.Context, fmisid int, period string) ([]weather.DailyClimateNormal, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT fmisid, period, month, day,
		       temp_avg, temp_high, temp_low,
		       feels_like_avg, feels_like_high, feels_like_low,
		       wind_avg, wind_gust, humidity_avg,
		       precip_mm, precip_days_pct, snow_cm,
		       temp_hourly, feels_like_hourly, wind_hourly, humidity_hourly,
		       temp_hourly_p10, temp_hourly_p90,
		       COALESCE(daily_years, 0), COALESCE(hourly_years, 0)
		FROM daily_climate_normals
		WHERE fmisid = $1 AND period = $2
		ORDER BY month, day`, fmisid, period)
	if err != nil {
		return nil, fmt.Errorf("get daily climate normals: %w", err)
	}
	defer rows.Close()

	var normals []weather.DailyClimateNormal
	for rows.Next() {
		var n weather.DailyClimateNormal
		if err := rows.Scan(&n.FMISID, &n.Period, &n.Month, &n.Day,
			&n.TempAvg, &n.TempHigh, &n.TempLow,
			&n.FeelsLikeAvg, &n.FeelsLikeHigh, &n.FeelsLikeLow,
			&n.WindAvg, &n.WindGust, &n.HumidityAvg,
			&n.PrecipMm, &n.PrecipDaysPct, &n.SnowCm,
			&n.TempHourly, &n.FeelsLikeHourly, &n.WindHourly, &n.HumidityHourly,
			&n.TempHourlyP10, &n.TempHourlyP90, &n.DailyYears, &n.HourlyYears); err != nil {
			return nil, fmt.Errorf("scan daily climate normal: %w", err)
		}
		normals = append(normals, n)
	}
	return normals, rows.Err()
}

// StationsWithClimateNormals lists the FMISIDs that have official monthly
// normals for the period.
func (s *Store) StationsWithClimateNormals(ctx context.Context, period string) ([]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT fmisid FROM climate_normals WHERE period = $1 ORDER BY fmisid`, period)
	if err != nil {
		return nil, fmt.Errorf("stations with climate normals: %w", err)
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan fmisid: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) GetClimateNormals(ctx context.Context, fmisid int, period string) ([]weather.ClimateNormal, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT fmisid, month, period, temp_avg, temp_high, temp_low, precip_mm
		FROM climate_normals
		WHERE fmisid = $1 AND period = $2
		ORDER BY month`, fmisid, period)
	if err != nil {
		return nil, fmt.Errorf("get climate normals: %w", err)
	}
	defer rows.Close()

	var normals []weather.ClimateNormal
	for rows.Next() {
		var n weather.ClimateNormal
		if err := rows.Scan(&n.FMISID, &n.Month, &n.Period, &n.TempAvg, &n.TempHigh, &n.TempLow, &n.PrecipMm); err != nil {
			return nil, fmt.Errorf("scan climate normal: %w", err)
		}
		normals = append(normals, n)
	}
	return normals, rows.Err()
}
