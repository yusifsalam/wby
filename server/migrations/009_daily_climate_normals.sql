CREATE TABLE IF NOT EXISTS daily_climate_normals (
    fmisid      INTEGER NOT NULL REFERENCES stations(fmisid),
    period      TEXT NOT NULL,
    month       SMALLINT NOT NULL CHECK (month BETWEEN 1 AND 12),
    day         SMALLINT NOT NULL CHECK (day BETWEEN 1 AND 31),
    temp_avg    DOUBLE PRECISION,
    temp_high   DOUBLE PRECISION,
    temp_low    DOUBLE PRECISION,
    precip_mm   DOUBLE PRECISION,
    temp_hourly DOUBLE PRECISION[],
    PRIMARY KEY (fmisid, period, month, day)
);
