package weather

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Calendar days are indexed as in a leap year so 29 February keeps its own
// slot and every other date maps to a fixed index regardless of year.
const calendarDays = 366

// Half-widths of the centred averaging windows, in days. Chosen against
// FMI's official 1991–2020 monthly normals for Kaisaniemi: ±7 keeps monthly
// temperature means within 0.05 °C and ±5 keeps monthly precipitation sums
// within ~1.5%, while the day-to-day jitter of the raw per-date means is
// already smoothed away. Wider precipitation windows flatten the August peak
// (±30 lost 13% of it).
const (
	normalsTempWindow   = 7
	normalsPrecipWindow = 5
	// Hourly samples pool each hour of each day within the window.
	normalsHourlyWindow = 7
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

// Hourly samples needed for a day to contribute a daily extreme (feels-like
// high/low, maximum gust).
const normalsMinHoursPerDay = 18

// NormalsMinHourlyYears is the record length, in years of the period, that
// hourly-derived normals (curves, feels-like, wind, humidity) require. Daily
// fields still need half the period.
const NormalsMinHourlyYears = 10

// Wet day threshold, mm. FMI marks dry days as -1 and traces as 0.
const normalsWetDayMm = 0.1

type hourlyCurve [24][calendarDays]normalAccumulator

func (c *hourlyCurve) at(index, minSamples int) []float64 {
	hours := make([]float64, 24)
	for h := 0; h < 24; h++ {
		v := c[h][index].mean(minSamples)
		if v == nil {
			return nil
		}
		hours[h] = *v
	}
	return hours
}

// hourlySamples keeps every pooled sample per hour and calendar day so
// percentiles can be taken once the pool is complete.
type hourlySamples [24][calendarDays][]float64

func (s *hourlySamples) add(hour, index int, v float64, halfWidth int) {
	for d := -halfWidth; d <= halfWidth; d++ {
		j := ((index+d)%calendarDays + calendarDays) % calendarDays
		s[hour][j] = append(s[hour][j], v)
	}
}

// percentiles returns the p-quantile curve for a day, or nil when any hour
// has fewer than minSamples. Sorting happens in place.
func (s *hourlySamples) percentiles(index, minSamples int, p float64) []float64 {
	hours := make([]float64, 24)
	for h := 0; h < 24; h++ {
		v := s[h][index]
		if len(v) < minSamples {
			return nil
		}
		slices.Sort(v)
		pos := p * float64(len(v)-1)
		lo := int(math.Floor(pos))
		hi := min(lo+1, len(v)-1)
		hours[h] = v[lo] + (v[hi]-v[lo])*(pos-float64(lo))
	}
	return hours
}

type dayExtremes struct {
	feelsHigh, feelsLow, gust float64
	feelsN, gustN             int
}

// ComputeDailyNormals derives a normal for every calendar day from daily and
// hourly observations within the period. Each day's value is the mean of all
// observations in a window centred on that day across every year of the
// period (15 days for most parameters, 11 for precipitation). A day is left
// nil when fewer than half the possible samples are present. Negative daily
// precipitation and snow depth, FMI's markers for none, count as zero.
//
// Feels-like is derived per hourly sample from temperature and wind; its
// daily high/low and the daily maximum gust come from days with at least
// normalsMinHoursPerDay samples.
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

	var avg, high, low, precip, wetDays, snow [calendarDays]normalAccumulator
	dailyDays, hourlyHours := 0, 0
	for _, r := range daily {
		if !inPeriod(r.Date) {
			continue
		}
		idx := calendarIndex(r.Date.UTC())
		if r.TempAvg != nil {
			dailyDays++
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
			wet := 0.0
			if *r.PrecipMm >= normalsWetDayMm {
				wet = 100
			}
			addWindowed(&wetDays, idx, wet, normalsPrecipWindow)
		}
		if r.SnowCm != nil {
			addWindowed(&snow, idx, max(*r.SnowCm, 0), normalsTempWindow)
		}
	}

	var tempCurve, feelsCurve, windCurve, humidityCurve hourlyCurve
	tempSamples := &hourlySamples{}
	var feelsAvg, feelsHigh, feelsLow, windAvg, gust, humidityAvg [calendarDays]normalAccumulator
	extremes := make(map[time.Time]*dayExtremes)
	for _, r := range hourly {
		if !inPeriod(r.Time) {
			continue
		}
		t := r.Time.UTC()
		idx := calendarIndex(t)
		h := t.Hour()
		if r.Temp != nil {
			hourlyHours++
			addWindowed(&tempCurve[h], idx, *r.Temp, normalsHourlyWindow)
			tempSamples.add(h, idx, *r.Temp, normalsHourlyWindow)
		}
		if r.Humidity != nil {
			addWindowed(&humidityCurve[h], idx, *r.Humidity, normalsHourlyWindow)
			addWindowed(&humidityAvg, idx, *r.Humidity, normalsTempWindow)
		}
		if r.WindSpeed != nil {
			addWindowed(&windCurve[h], idx, *r.WindSpeed, normalsHourlyWindow)
			addWindowed(&windAvg, idx, *r.WindSpeed, normalsTempWindow)
		}
		if r.Temp == nil && r.WindGust == nil {
			continue
		}
		date := t.Truncate(24 * time.Hour)
		e, ok := extremes[date]
		if !ok {
			e = &dayExtremes{}
			extremes[date] = e
		}
		if r.Temp != nil && r.WindSpeed != nil {
			f := *FeelsLike(r.Temp, r.WindSpeed)
			addWindowed(&feelsCurve[h], idx, f, normalsHourlyWindow)
			addWindowed(&feelsAvg, idx, f, normalsTempWindow)
			if e.feelsN == 0 || f > e.feelsHigh {
				e.feelsHigh = f
			}
			if e.feelsN == 0 || f < e.feelsLow {
				e.feelsLow = f
			}
			e.feelsN++
		}
		if r.WindGust != nil {
			if e.gustN == 0 || *r.WindGust > e.gust {
				e.gust = *r.WindGust
			}
			e.gustN++
		}
	}
	for date, e := range extremes {
		idx := calendarIndex(date)
		if e.feelsN >= normalsMinHoursPerDay {
			addWindowed(&feelsHigh, idx, e.feelsHigh, normalsTempWindow)
			addWindowed(&feelsLow, idx, e.feelsLow, normalsTempWindow)
		}
		if e.gustN >= normalsMinHoursPerDay {
			addWindowed(&gust, idx, e.gust, normalsTempWindow)
		}
	}

	minDaily := (2*normalsTempWindow + 1) * years / 2
	minPrecip := (2*normalsPrecipWindow + 1) * years / 2
	hourlyYearsNeeded := min(years, NormalsMinHourlyYears)
	minHourly := (2*normalsHourlyWindow + 1) * hourlyYearsNeeded
	minHourlyDays := (2*normalsTempWindow + 1) * hourlyYearsNeeded
	minAllHours := minHourlyDays * 24
	dailyYears := float64(dailyDays) / 365.25
	hourlyYears := float64(hourlyHours) / 8766

	out := make([]DailyClimateNormal, 0, calendarDays)
	for i := 0; i < calendarDays; i++ {
		month, day := calendarDate(i)
		out = append(out, DailyClimateNormal{
			FMISID:          fmisid,
			Period:          period,
			Month:           month,
			Day:             day,
			TempAvg:         avg[i].mean(minDaily),
			TempHigh:        high[i].mean(minDaily),
			TempLow:         low[i].mean(minDaily),
			FeelsLikeAvg:    feelsAvg[i].mean(minAllHours),
			FeelsLikeHigh:   feelsHigh[i].mean(minHourlyDays),
			FeelsLikeLow:    feelsLow[i].mean(minHourlyDays),
			WindAvg:         windAvg[i].mean(minAllHours),
			WindGust:        gust[i].mean(minHourlyDays),
			HumidityAvg:     humidityAvg[i].mean(minAllHours),
			PrecipMm:        precip[i].mean(minPrecip),
			PrecipDaysPct:   wetDays[i].mean(minPrecip),
			SnowCm:          snow[i].mean(minDaily),
			TempHourly:      tempCurve.at(i, minHourly),
			TempHourlyP10:   tempSamples.percentiles(i, minHourly, 0.1),
			TempHourlyP90:   tempSamples.percentiles(i, minHourly, 0.9),
			FeelsLikeHourly: feelsCurve.at(i, minHourly),
			WindHourly:      windCurve.at(i, minHourly),
			HumidityHourly:  humidityCurve.at(i, minHourly),
			DailyYears:      dailyYears,
			HourlyYears:     hourlyYears,
		})
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
