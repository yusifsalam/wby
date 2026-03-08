package weather

import (
	"math"
	"time"
)

// InterpolateNormals performs cosine interpolation of monthly climate normals
// for a given date. Each monthly normal is placed at the 15th of its month.
// For a given date, the two surrounding mid-month points are found and
// cosine-interpolated to produce a smooth daily curve.
func InterpolateNormals(normals []ClimateNormal, date time.Time, currentTemp *float64) InterpolatedNormal {
	if len(normals) != 12 {
		return InterpolatedNormal{}
	}

	// Index normals by month (1-12 -> 0-11).
	byMonth := make([]*ClimateNormal, 12)
	for i := range normals {
		m := normals[i].Month
		if m < 1 || m > 12 {
			return InterpolatedNormal{}
		}
		byMonth[m-1] = &normals[i]
	}

	// Determine the two surrounding mid-month anchor points.
	year := date.Year()
	month := date.Month()
	day := date.Day()

	// Mid-month for current month is the 15th.
	midCurrent := time.Date(year, month, 15, 0, 0, 0, 0, time.UTC)

	var midBefore, midAfter time.Time
	var monthBefore, monthAfter int // 0-based index

	if day < 15 {
		// Date is before mid-month: interpolate between previous month's 15th and current month's 15th.
		midAfter = midCurrent
		monthAfter = int(month) - 1 // 0-based

		prevMonth := month - 1
		prevYear := year
		if prevMonth < 1 {
			prevMonth = 12
			prevYear--
		}
		midBefore = time.Date(prevYear, prevMonth, 15, 0, 0, 0, 0, time.UTC)
		monthBefore = int(prevMonth) - 1 // 0-based
	} else {
		// Date is on or after mid-month: interpolate between current month's 15th and next month's 15th.
		midBefore = midCurrent
		monthBefore = int(month) - 1 // 0-based

		nextMonth := month + 1
		nextYear := year
		if nextMonth > 12 {
			nextMonth = 1
			nextYear++
		}
		midAfter = time.Date(nextYear, nextMonth, 15, 0, 0, 0, 0, time.UTC)
		monthAfter = int(nextMonth) - 1 // 0-based
	}

	// Compute the interpolation parameter t in [0, 1].
	totalDuration := midAfter.Sub(midBefore).Seconds()
	elapsed := date.Sub(midBefore).Seconds()
	t := elapsed / totalDuration

	// Cosine interpolation weight.
	weight := (1 - math.Cos(t*math.Pi)) / 2

	before := byMonth[monthBefore]
	after := byMonth[monthAfter]

	var result InterpolatedNormal

	result.TempAvg = cosineInterp(before.TempAvg, after.TempAvg, weight)
	result.TempHigh = cosineInterp(before.TempHigh, after.TempHigh, weight)
	result.TempLow = cosineInterp(before.TempLow, after.TempLow, weight)

	// For precipitation: interpolate the monthly total, then divide by days in the current month.
	interpPrecip := cosineInterp(before.PrecipMm, after.PrecipMm, weight)
	if interpPrecip != nil {
		daysInMonth := daysIn(date.Year(), date.Month())
		v := *interpPrecip / float64(daysInMonth)
		result.PrecipMmDay = &v
	}

	// Compute TempDiff = currentTemp - interpolated TempAvg.
	if currentTemp != nil && result.TempAvg != nil {
		diff := *currentTemp - *result.TempAvg
		result.TempDiff = &diff
	}

	return result
}

// cosineInterp interpolates between two optional float64 values using a
// precomputed cosine weight. Returns nil if either value is nil.
func cosineInterp(a, b *float64, weight float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a*(1-weight) + *b*weight
	return &v
}

// daysIn returns the number of days in the given month of the given year.
func daysIn(year int, month time.Month) int {
	// The zeroth day of the next month is the last day of the current month.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
