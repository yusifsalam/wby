package weather

import (
	"maps"
	"time"
)

// Candidates within stationSearchRadiusKm are ranked by distance plus a
// per-missing-parameter penalty in kilometres; gaps in the winner's
// observation are filled from the nearest station reporting them.
const (
	stationSearchRadiusKm = 30.0
	stationCandidateLimit = 8
	observationMaxAge     = 2 * time.Hour
	observationExpiry     = 3 * time.Hour

	penaltyMissingTemperatureKm = 15.0
	penaltyMissingWeatherKm     = 10.0
	penaltyMissingWindKm        = 1.0
	penaltyMissingHumidityKm    = 1.0
)

// stationRankKm is the effective distance used to rank a candidate.
func stationRankKm(sd StationDistance, o Observation) float64 {
	km := sd.DistanceKM
	if o.Temperature == nil {
		km += penaltyMissingTemperatureKm
	}
	if o.WeatherCode == nil && o.TotalCloudCover == nil {
		km += penaltyMissingWeatherKm
	}
	if o.WindSpeed == nil {
		km += penaltyMissingWindKm
	}
	if o.Humidity == nil {
		km += penaltyMissingHumidityKm
	}
	return km
}

// pickStation returns the best-ranked fresh candidate (candidates are nearest
// first) with its observation backfilled from the other fresh ones in distance
// order, and the fields each of those supplied. Without any fresh observation
// it returns the nearest candidate that has one, unmerged and without its
// values once they are older than observationExpiry; ok is false when none
// has any.
func pickStation(candidates []StationDistance, obs map[int]Observation, now time.Time) (StationDistance, Observation, []BorrowedValues, bool) {
	var fresh []StationDistance
	for _, c := range candidates {
		o, ok := obs[c.FMISID]
		if ok && now.Sub(o.ObservedAt) <= observationMaxAge {
			fresh = append(fresh, c)
		}
	}
	if len(fresh) == 0 {
		for _, c := range candidates {
			if o, ok := obs[c.FMISID]; ok {
				return c, withoutExpiredValues(o, now), nil, true
			}
		}
		return StationDistance{}, Observation{}, nil, false
	}

	best := fresh[0]
	bestKm := stationRankKm(best, obs[best.FMISID])
	for _, c := range fresh[1:] {
		if km := stationRankKm(c, obs[c.FMISID]); km < bestKm {
			best, bestKm = c, km
		}
	}

	merged := obs[best.FMISID]
	merged.ExtraNumericParams = maps.Clone(merged.ExtraNumericParams)
	var borrowed []BorrowedValues
	for _, c := range fresh {
		if c.FMISID == best.FMISID {
			continue
		}
		if fields := fillObservation(&merged, obs[c.FMISID]); len(fields) > 0 {
			borrowed = append(borrowed, BorrowedValues{Station: c.Station, DistanceKM: c.DistanceKM, Fields: fields})
		}
	}
	return best, merged, borrowed, true
}

// withoutExpiredValues returns o, stripped to its station and timestamp when
// it is older than observationExpiry.
func withoutExpiredValues(o Observation, now time.Time) Observation {
	if now.Sub(o.ObservedAt) <= observationExpiry {
		return o
	}
	return Observation{FMISID: o.FMISID, ObservedAt: o.ObservedAt}
}

// fillObservation copies each parameter that dst lacks from src and returns
// the names of the ones it copied, extras excluded.
func fillObservation(dst *Observation, src Observation) []string {
	var filled []string
	fill := func(name string, d **float64, s *float64) {
		if *d == nil && s != nil {
			*d = s
			filled = append(filled, name)
		}
	}
	fill("temperature", &dst.Temperature, src.Temperature)
	fill("wind_speed", &dst.WindSpeed, src.WindSpeed)
	fill("wind_gust", &dst.WindGust, src.WindGust)
	fill("wind_direction", &dst.WindDir, src.WindDir)
	fill("humidity", &dst.Humidity, src.Humidity)
	fill("dew_point", &dst.DewPoint, src.DewPoint)
	fill("pressure", &dst.Pressure, src.Pressure)
	fill("precipitation_1h", &dst.Precip1h, src.Precip1h)
	fill("precipitation_intensity", &dst.PrecipIntensity, src.PrecipIntensity)
	fill("snow_depth", &dst.SnowDepth, src.SnowDepth)
	fill("visibility", &dst.Visibility, src.Visibility)
	fill("cloud_cover", &dst.TotalCloudCover, src.TotalCloudCover)
	fill("weather_code", &dst.WeatherCode, src.WeatherCode)
	for k, v := range src.ExtraNumericParams {
		if _, ok := dst.ExtraNumericParams[k]; ok {
			continue
		}
		if dst.ExtraNumericParams == nil {
			dst.ExtraNumericParams = map[string]float64{}
		}
		dst.ExtraNumericParams[k] = v
	}
	return filled
}
