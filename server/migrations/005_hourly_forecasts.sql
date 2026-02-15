CREATE TABLE IF NOT EXISTS hourly_forecasts (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    grid_lat         DOUBLE PRECISION NOT NULL,
    grid_lon         DOUBLE PRECISION NOT NULL,
    forecast_time    TIMESTAMPTZ NOT NULL,
    fetched_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    temperature      DOUBLE PRECISION,
    wind_speed       DOUBLE PRECISION,
    wind_direction   DOUBLE PRECISION,
    humidity         DOUBLE PRECISION,
    precipitation_1h DOUBLE PRECISION,
    symbol           TEXT,
    UNIQUE (grid_lat, grid_lon, forecast_time)
);

CREATE INDEX IF NOT EXISTS idx_hourly_forecasts_grid_time
    ON hourly_forecasts (grid_lat, grid_lon, forecast_time);
