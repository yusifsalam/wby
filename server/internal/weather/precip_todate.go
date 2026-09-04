package weather

import (
	"context"
	"time"
)

// PrecipitationHistoryFetcher supplies hourly precipitation observations from
// the gauge nearest a point, for month-to-date comparisons. Optional on the
// FMI client.
type PrecipitationHistoryFetcher interface {
	FetchHourlyPrecipitationNear(ctx context.Context, lat, lon float64, start, end time.Time) (PrecipitationObservations, error)
}

// precipitationToDate sums observed hourly accumulations since local midnight
// and since the first of the month, and the daily normals over the same days.
// hourly records carry the accumulation for the hour ending at their time, so
// a record stamped exactly at a boundary belongs to the period before it.
func precipitationToDate(normals []DailyClimateNormal, hourly []HourlyRecord, now time.Time, loc *time.Location) PrecipitationToDate {
	local := now.In(loc)
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	monthStart := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)

	var out PrecipitationToDate
	var today, month float64
	observed := false
	for _, r := range hourly {
		if r.PrecipMm == nil || !r.Time.After(monthStart) || r.Time.After(now) {
			continue
		}
		observed = true
		v := max(*r.PrecipMm, 0)
		month += v
		if r.Time.After(dayStart) {
			today += v
		}
		if out.ObservedThrough == nil || r.Time.After(*out.ObservedThrough) {
			t := r.Time
			out.ObservedThrough = &t
		}
	}
	if observed {
		out.TodayObservedMm = &today
		out.MonthToDateObservedMm = &month
	}

	daysInMonth := time.Date(local.Year(), local.Month()+1, 0, 0, 0, 0, 0, loc).Day()
	var toDate, whole float64
	toDateN, wholeN := 0, 0
	for _, n := range normals {
		if n.Month != int(local.Month()) || n.PrecipMm == nil || n.Day > daysInMonth {
			continue
		}
		whole += *n.PrecipMm
		wholeN++
		if n.Day <= local.Day() {
			toDate += *n.PrecipMm
			toDateN++
		}
		if n.Day == local.Day() {
			v := *n.PrecipMm
			out.TodayNormalMm = &v
		}
	}
	if toDateN == local.Day() {
		out.MonthToDateNormalMm = &toDate
	}
	if wholeN == daysInMonth {
		out.MonthNormalMm = &whole
	}
	return out
}
