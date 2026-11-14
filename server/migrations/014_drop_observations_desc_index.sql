-- UNIQUE (fmisid, observed_at) already indexes the same columns and serves
-- every query this index did. CONCURRENTLY avoids blocking reads and writes on
-- observations while the index is removed.
DROP INDEX CONCURRENTLY IF EXISTS idx_observations_fmisid_time;
