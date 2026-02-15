CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE stations (
    fmisid    INTEGER PRIMARY KEY,
    name      TEXT NOT NULL,
    geom      GEOGRAPHY(POINT, 4326) NOT NULL,
    wmo_code  TEXT
);

CREATE INDEX idx_stations_geom ON stations USING GIST (geom);

CREATE TABLE observations (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    fmisid      INTEGER NOT NULL REFERENCES stations(fmisid),
    observed_at TIMESTAMPTZ NOT NULL,
    temperature DOUBLE PRECISION,
    wind_speed  DOUBLE PRECISION,
    wind_dir    DOUBLE PRECISION,
    humidity    DOUBLE PRECISION,
    pressure    DOUBLE PRECISION,
    UNIQUE (fmisid, observed_at)
);

CREATE INDEX idx_observations_fmisid_time ON observations (fmisid, observed_at DESC);

CREATE TABLE forecasts (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    grid_lat     DOUBLE PRECISION NOT NULL,
    grid_lon     DOUBLE PRECISION NOT NULL,
    forecast_for DATE NOT NULL,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    temp_high    DOUBLE PRECISION,
    temp_low     DOUBLE PRECISION,
    wind_speed   DOUBLE PRECISION,
    precip_mm    DOUBLE PRECISION,
    symbol       TEXT,
    UNIQUE (grid_lat, grid_lon, forecast_for)
);

CREATE INDEX idx_forecasts_grid_date ON forecasts (grid_lat, grid_lon, forecast_for);
