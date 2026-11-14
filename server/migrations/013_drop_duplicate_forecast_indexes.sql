-- Both indexes repeat the columns of the table's UNIQUE constraint, whose own
-- index serves the same lookups.
DROP INDEX IF EXISTS idx_forecasts_grid_date;
DROP INDEX IF EXISTS idx_hourly_forecasts_grid_time;
