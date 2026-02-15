package store

import (
	"context"
	"fmt"

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
		batch.Queue(
			`INSERT INTO observations (fmisid, observed_at, temperature, wind_speed, wind_dir, humidity, pressure)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (fmisid, observed_at) DO UPDATE SET
			   temperature = $3, wind_speed = $4, wind_dir = $5, humidity = $6, pressure = $7`,
			o.FMISID, o.ObservedAt, o.Temperature, o.WindSpeed, o.WindDir, o.Humidity, o.Pressure,
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
	err := s.pool.QueryRow(ctx,
		`SELECT fmisid, observed_at, temperature, wind_speed, wind_dir, humidity, pressure
		 FROM observations
		 WHERE fmisid = $1
		 ORDER BY observed_at DESC
		 LIMIT 1`,
		fmisid,
	).Scan(&o.FMISID, &o.ObservedAt, &o.Temperature, &o.WindSpeed, &o.WindDir, &o.Humidity, &o.Pressure)
	if err != nil {
		return o, fmt.Errorf("latest observation: %w", err)
	}
	return o, nil
}

func (s *Store) UpsertForecasts(ctx context.Context, forecasts []weather.DailyForecast) error {
	batch := &pgx.Batch{}
	for _, f := range forecasts {
		batch.Queue(
			`INSERT INTO forecasts (grid_lat, grid_lon, forecast_for, fetched_at, temp_high, temp_low, wind_speed, precip_mm, symbol)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 ON CONFLICT (grid_lat, grid_lon, forecast_for) DO UPDATE SET
			   fetched_at = $4, temp_high = $5, temp_low = $6, wind_speed = $7, precip_mm = $8, symbol = $9`,
			f.GridLat, f.GridLon, f.Date, f.FetchedAt, f.TempHigh, f.TempLow, f.WindSpeed, f.PrecipMM, f.Symbol,
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
		`SELECT grid_lat, grid_lon, forecast_for, fetched_at, temp_high, temp_low, wind_speed, precip_mm, symbol
		 FROM forecasts
		 WHERE grid_lat = $1 AND grid_lon = $2 AND forecast_for >= CURRENT_DATE
		 ORDER BY forecast_for`,
		gridLat, gridLon,
	)
	if err != nil {
		return nil, fmt.Errorf("get forecasts: %w", err)
	}
	defer rows.Close()

	var result []weather.DailyForecast
	for rows.Next() {
		var f weather.DailyForecast
		if err := rows.Scan(&f.GridLat, &f.GridLon, &f.Date, &f.FetchedAt, &f.TempHigh, &f.TempLow, &f.WindSpeed, &f.PrecipMM, &f.Symbol); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, nil
}
