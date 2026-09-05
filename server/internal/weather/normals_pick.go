package weather

// normalsCoveragePenaltyKmPerYear converts a candidate's missing record
// length into distance so that a longer record wins over a nearer station
// unless the gap is under about a year.
const normalsCoveragePenaltyKmPerYear = 3.0

func normalsRankKm(distKm, years float64, periodYears int) float64 {
	return distKm + max(0, float64(periodYears)-years)*normalsCoveragePenaltyKmPerYear
}

// pickNormalsStations chooses the station whose daily record represents the
// location and, when that station has no hourly curves for the day, the
// station supplying the hourly-derived fields. hourly is nil when the primary
// covers both or no candidate has curves; primary is nil when no candidate
// has a temperature normal.
func pickNormalsStations(candidates []DailyNormalsCandidate, periodYears int) (primary, hourly *DailyNormalsCandidate) {
	var bestKm, bestHourlyKm float64
	for i := range candidates {
		c := &candidates[i]
		if c.HasTemp {
			if km := normalsRankKm(c.DistanceKM, c.DailyYears, periodYears); primary == nil || km < bestKm {
				primary, bestKm = c, km
			}
		}
		if c.HasHourly {
			if km := normalsRankKm(c.DistanceKM, c.HourlyYears, periodYears); hourly == nil || km < bestHourlyKm {
				hourly, bestHourlyKm = c, km
			}
		}
	}
	if primary == nil || primary.HasHourly || (hourly != nil && hourly.FMISID == primary.FMISID) {
		return primary, nil
	}
	return primary, hourly
}

// mergeHourlyNormals copies the hourly-derived fields of src into dst for
// each calendar day, leaving the daily-record fields of dst untouched.
// Temperature-level fields are shifted so the borrowed curve's mean matches
// dst's daily mean: src lends its diurnal shape, dst keeps its level.
func mergeHourlyNormals(dst, src []DailyClimateNormal) {
	type key struct{ month, day int }
	byDay := make(map[key]DailyClimateNormal, len(src))
	for _, n := range src {
		byDay[key{n.Month, n.Day}] = n
	}
	for i := range dst {
		n, ok := byDay[key{dst[i].Month, dst[i].Day}]
		if !ok {
			continue
		}
		d := &dst[i]
		shift := 0.0
		if d.TempAvg != nil && len(n.TempHourly) == 24 {
			shift = *d.TempAvg - mean(n.TempHourly)
		}
		d.FeelsLikeAvg, d.FeelsLikeHigh, d.FeelsLikeLow = shifted(n.FeelsLikeAvg, shift), shifted(n.FeelsLikeHigh, shift), shifted(n.FeelsLikeLow, shift)
		d.WindAvg, d.WindGust, d.HumidityAvg = n.WindAvg, n.WindGust, n.HumidityAvg
		d.TempHourly, d.TempHourlyP10, d.TempHourlyP90 = shiftedCurve(n.TempHourly, shift), shiftedCurve(n.TempHourlyP10, shift), shiftedCurve(n.TempHourlyP90, shift)
		d.FeelsLikeHourly, d.WindHourly, d.HumidityHourly = shiftedCurve(n.FeelsLikeHourly, shift), n.WindHourly, n.HumidityHourly
		d.HourlyYears = n.HourlyYears
	}
}

func mean(v []float64) float64 {
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}

func shifted(v *float64, by float64) *float64 {
	if v == nil {
		return nil
	}
	out := *v + by
	return &out
}

func shiftedCurve(v []float64, by float64) []float64 {
	if v == nil {
		return nil
	}
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x + by
	}
	return out
}
