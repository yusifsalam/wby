package weather

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Calendar days are indexed as in a leap year so 29 February keeps its own
// slot and every other date maps to a fixed index regardless of year.
const calendarDays = 366

const (
	// Half-widths of the centred averaging windows, in days.
	normalsTempWindow   = 15
	normalsPrecipWindow = 30
	// Half-width of the tolerance for hourly samples: each hour of each day
	// within the window across the period.
	normalsHourlyWindow = 15
)

type normalAccumulator struct {
	sum float64
	n   int
}

func (a *normalAccumulator) add(v float64) {
	a.sum += v
	a.n++
}

func (a normalAccumulator) mean(minSamples int) *float64 {
	if a.n < minSamples {
		return nil
	}
	v := a.sum / float64(a.n)
	return &v
}

func calendarIndex(t time.Time) int {
	return time.Date(2000, t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).YearDay() - 1
}

func calendarDate(index int) (month, day int) {
	d := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, index)
	return int(d.Month()), d.Day()
}

func parsePeriod(period string) (startYear, endYear int, err error) {
	parts := strings.SplitN(period, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("period %q: want YYYY-YYYY", period)
	}
	startYear, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("period %q: %w", period, err)
	}
	endYear, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("period %q: %w", period, err)
	}
	if endYear < startYear {
		return 0, 0, fmt.Errorf("period %q: end before start", period)
	}
	return startYear, endYear, nil
}

func addWindowed(acc *[calendarDays]normalAccumulator, index int, v float64, halfWidth int) {
	for d := -halfWidth; d <= halfWidth; d++ {
		acc[((index+d)%calendarDays+calendarDays)%calendarDays].add(v)
	}
}

// ComputeDailyNormals derives a normal for every calendar day from daily and
// hourly observations within the period. Each day's value is the mean of all
// observations in a window centred on that day across every year of the
// period (31 days for temperature, 61 for precipitation). A day is left nil
// when fewer than half the possible samples are present. Negative daily
// precipitation, FMI's marker for a dry day, counts as zero.
func ComputeDailyNormals(fmisid int, period string, daily []DailyRecord, hourly []HourlyRecord) ([]DailyClimateNormal, error) {
	startYear, endYear, err := parsePeriod(period)
	if err != nil {
		return nil, err
	}
	years := endYear - startYear + 1
	inPeriod := func(t time.Time) bool {
		y := t.UTC().Year()
		return y >= startYear && y <= endYear
	}

	var avg, high, low, precip [calendarDays]normalAccumulator
	for _, r := range daily {
		if !inPeriod(r.Date) {
			continue
		}
		idx := calendarIndex(r.Date.UTC())
		if r.TempAvg != nil {
			addWindowed(&avg, idx, *r.TempAvg, normalsTempWindow)
		}
		if r.TempHigh != nil {
			addWindowed(&high, idx, *r.TempHigh, normalsTempWindow)
		}
		if r.TempLow != nil {
			addWindowed(&low, idx, *r.TempLow, normalsTempWindow)
		}
		if r.PrecipMm != nil {
			addWindowed(&precip, idx, max(*r.PrecipMm, 0), normalsPrecipWindow)
		}
	}

	var hourlyAcc [24][calendarDays]normalAccumulator
	for _, r := range hourly {
		if !inPeriod(r.Time) {
			continue
		}
		t := r.Time.UTC()
		addWindowed(&hourlyAcc[t.Hour()], calendarIndex(t), r.Temp, normalsHourlyWindow)
	}

	minTemp := (2*normalsTempWindow + 1) * years / 2
	minPrecip := (2*normalsPrecipWindow + 1) * years / 2
	minHourly := (2*normalsHourlyWindow + 1) * years / 2

	out := make([]DailyClimateNormal, 0, calendarDays)
	for i := 0; i < calendarDays; i++ {
		month, day := calendarDate(i)
		n := DailyClimateNormal{
			FMISID:   fmisid,
			Period:   period,
			Month:    month,
			Day:      day,
			TempAvg:  avg[i].mean(minTemp),
			TempHigh: high[i].mean(minTemp),
			TempLow:  low[i].mean(minTemp),
			PrecipMm: precip[i].mean(minPrecip),
		}
		hours := make([]float64, 24)
		complete := true
		for h := 0; h < 24; h++ {
			v := hourlyAcc[h][i].mean(minHourly)
			if v == nil {
				complete = false
				break
			}
			hours[h] = *v
		}
		if complete {
			n.TempHourly = hours
		}
		out = append(out, n)
	}
	return out, nil
}

// HourlyNormalAt linearly interpolates a 24-entry UTC hourly normal curve at
// the given instant. next supplies hour 0 of the following day for the last
// hour; when nil the curve wraps onto itself.
func HourlyNormalAt(hours, next []float64, at time.Time) *float64 {
	if len(hours) != 24 {
		return nil
	}
	utc := at.UTC()
	h := utc.Hour()
	frac := float64(utc.Minute())/60 + float64(utc.Second())/3600
	a := hours[h]
	var b float64
	switch {
	case h < 23:
		b = hours[h+1]
	case len(next) == 24:
		b = next[0]
	default:
		b = hours[0]
	}
	v := a + (b-a)*frac
	return &v
}
