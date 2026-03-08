CREATE TABLE IF NOT EXISTS climate_normals (
    fmisid    INTEGER NOT NULL REFERENCES stations(fmisid),
    month     SMALLINT NOT NULL CHECK (month BETWEEN 1 AND 12),
    period    TEXT NOT NULL DEFAULT '1991-2020',
    temp_avg  DOUBLE PRECISION,
    temp_high DOUBLE PRECISION,
    temp_low  DOUBLE PRECISION,
    precip_mm DOUBLE PRECISION,
    PRIMARY KEY (fmisid, month, period)
);
