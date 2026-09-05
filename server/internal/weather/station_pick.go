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
// order. Without any fresh observation it returns the nearest candidate that
// has one, unmerged; ok is false when none has any.
func pickStation(candidates []StationDistance, obs map[int]Observation, now time.Time) (StationDistance, Observation, bool) {
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
				return c, o, true
			}
		}
		return StationDistance{}, Observation{}, false
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
	for _, c := range fresh {
		if c.FMISID == best.FMISID {
			continue
		}
		fillObservation(&merged, obs[c.FMISID])
	}
	return best, merged, true
}

// fillObservation copies each parameter that dst lacks from src.
func fillObservation(dst *Observation, src Observation) {
	fill := func(d **float64, s *float64) {
		if *d == nil && s != nil {
			*d = s
		}
	}
	fill(&dst.Temperature, src.Temperature)
	fill(&dst.WindSpeed, src.WindSpeed)
	fill(&dst.WindGust, src.WindGust)
	fill(&dst.WindDir, src.WindDir)
	fill(&dst.Humidity, src.Humidity)
	fill(&dst.DewPoint, src.DewPoint)
	fill(&dst.Pressure, src.Pressure)
	fill(&dst.Precip1h, src.Precip1h)
	fill(&dst.PrecipIntensity, src.PrecipIntensity)
	fill(&dst.SnowDepth, src.SnowDepth)
	fill(&dst.Visibility, src.Visibility)
	fill(&dst.TotalCloudCover, src.TotalCloudCover)
	fill(&dst.WeatherCode, src.WeatherCode)
	for k, v := range src.ExtraNumericParams {
		if _, ok := dst.ExtraNumericParams[k]; ok {
			continue
		}
		if dst.ExtraNumericParams == nil {
			dst.ExtraNumericParams = map[string]float64{}
		}
		dst.ExtraNumericParams[k] = v
	}
}
