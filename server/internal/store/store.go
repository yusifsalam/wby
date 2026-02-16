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

func (s *Store) LatestObservation(ctx context.Context, fmisid int) (weather.Observation, error) {
	var o weather.Observation
	var extraRaw []byte
	err := s.pool.QueryRow(ctx,
		`SELECT fmisid, observed_at, temperature, wind_speed, wind_gust, wind_dir, humidity, dew_point,
		        pressure, precip_1h, precip_intensity, snow_depth, visibility, total_cloud_cover, weather_code, extra
		 FROM observations
		 WHERE fmisid = $1
		 ORDER BY observed_at DESC
		 LIMIT 1`,
		fmisid,
	).Scan(
		&o.FMISID, &o.ObservedAt, &o.Temperature, &o.WindSpeed, &o.WindGust, &o.WindDir, &o.Humidity, &o.DewPoint,
		&o.Pressure, &o.Precip1h, &o.PrecipIntensity, &o.SnowDepth, &o.Visibility, &o.TotalCloudCover, &o.WeatherCode, &extraRaw,
	)
	if err != nil {
		return o, fmt.Errorf("latest observation: %w", err)
	}
	o.ExtraNumericParams = decodeNumericExtras(extraRaw)
	return o, nil
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
				temperature, wind_speed, wind_direction, humidity, precipitation_1h, symbol, uv_cumulated
			)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			 ON CONFLICT (grid_lat, grid_lon, forecast_time) DO UPDATE SET
			   fetched_at = $4, temperature = $5, wind_speed = $6, wind_direction = $7,
			   humidity = $8, precipitation_1h = $9, symbol = $10, uv_cumulated = $11`,
			gridLat, gridLon, h.Time, fetchedAt,
			h.Temperature, h.WindSpeed, h.WindDir, h.Humidity, h.Precip1h, h.Symbol, h.UVCumulated,
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
		`SELECT forecast_time, fetched_at, temperature, wind_speed, wind_direction, humidity, precipitation_1h, symbol, uv_cumulated
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
		); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, nil
}
