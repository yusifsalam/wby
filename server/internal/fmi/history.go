package fmi

import (
	"context"
	"encoding/xml"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"wby/internal/weather"
)

// FMI caps a single daily request at one year and an hourly request at 31
// days, so historical ranges are fetched in chunks.
const (
	dailyObservationChunk  = 365 * 24 * time.Hour
	hourlyObservationChunk = 31 * 24 * time.Hour
	historyConcurrency     = 4
)

// FetchDailyObservations returns the daily mean/max/min temperature and
// precipitation for a station between start and end (inclusive).
func (c *Client) FetchDailyObservations(ctx context.Context, fmisid int, start, end time.Time) ([]weather.DailyRecord, error) {
	chunks, err := c.fetchChunked(ctx, "fmi::observations::weather::daily::timevaluepair", "tday,tmax,tmin,rrday", fmisid, start, end, dailyObservationChunk)
	if err != nil {
		return nil, fmt.Errorf("fetch daily observations: %w", err)
	}
	byDate := make(map[time.Time]weather.DailyRecord)
	for _, data := range chunks {
		recs, err := ParseDailyObservations(data)
		if err != nil {
			return nil, err
		}
		for _, r := range recs {
			byDate[r.Date] = r
		}
	}
	out := make([]weather.DailyRecord, 0, len(byDate))
	for _, r := range byDate {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b weather.DailyRecord) int { return a.Date.Compare(b.Date) })
	return out, nil
}

// FetchHourlyObservations returns the hourly mean temperature for a station
// between start and end (inclusive).
func (c *Client) FetchHourlyObservations(ctx context.Context, fmisid int, start, end time.Time) ([]weather.HourlyRecord, error) {
	chunks, err := c.fetchChunked(ctx, "fmi::observations::weather::hourly::timevaluepair", "TA_PT1H_AVG", fmisid, start, end, hourlyObservationChunk)
	if err != nil {
		return nil, fmt.Errorf("fetch hourly observations: %w", err)
	}
	byTime := make(map[time.Time]float64)
	for _, data := range chunks {
		recs, err := ParseHourlyObservations(data)
		if err != nil {
			return nil, err
		}
		for _, r := range recs {
			byTime[r.Time] = r.Temp
		}
	}
	out := make([]weather.HourlyRecord, 0, len(byTime))
	for t, v := range byTime {
		out = append(out, weather.HourlyRecord{Time: t, Temp: v})
	}
	slices.SortFunc(out, func(a, b weather.HourlyRecord) int { return a.Time.Compare(b.Time) })
	return out, nil
}

func (c *Client) fetchChunked(ctx context.Context, query, parameters string, fmisid int, start, end time.Time, chunk time.Duration) ([][]byte, error) {
	type span struct{ start, end time.Time }
	var spans []span
	for cur := start; !cur.After(end); cur = cur.Add(chunk) {
		next := cur.Add(chunk)
		if next.After(end) {
			next = end
		}
		spans = append(spans, span{cur, next})
	}

	results := make([][]byte, len(spans))
	errs := make([]error, len(spans))
	sem := make(chan struct{}, historyConcurrency)
	var wg sync.WaitGroup
	for i, sp := range spans {
		wg.Add(1)
		go func(i int, sp span) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			params := url.Values{
				"service":        {"WFS"},
				"version":        {"2.0.0"},
				"request":        {"getFeature"},
				"storedquery_id": {query},
				"fmisid":         {strconv.Itoa(fmisid)},
				"parameters":     {parameters},
				"starttime":      {sp.start.UTC().Format(time.RFC3339)},
				"endtime":        {sp.end.UTC().Format(time.RFC3339)},
			}
			results[i], errs[i] = c.fetch(ctx, params)
		}(i, sp)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("%s..%s: %w", spans[i].start.Format("2006-01-02"), spans[i].end.Format("2006-01-02"), err)
		}
	}
	return results, nil
}

// ParseDailyObservations parses a daily observation timevaluepair response
// into one record per date.
func ParseDailyObservations(data []byte) ([]weather.DailyRecord, error) {
	var fc featureCollection
	if err := xml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("unmarshal WFS daily observations: %w", err)
	}

	byDate := make(map[time.Time]*weather.DailyRecord)
	for _, m := range fc.Members {
		param := strings.ToLower(extractParam(m.Observation.ObservedProperty.Href))
		for _, pt := range m.Observation.Result.TimeSeries.Points {
			t, err := time.Parse(time.RFC3339, pt.TVP.Time)
			if err != nil {
				continue
			}
			val := parseFloat(pt.TVP.Value)
			if val == nil || math.IsNaN(*val) {
				continue
			}
			date := t.UTC().Truncate(24 * time.Hour)
			r, ok := byDate[date]
			if !ok {
				r = &weather.DailyRecord{Date: date}
				byDate[date] = r
			}
			switch param {
			case "tday":
				r.TempAvg = val
			case "tmax":
				r.TempHigh = val
			case "tmin":
				r.TempLow = val
			case "rrday":
				r.PrecipMm = val
			}
		}
	}

	out := make([]weather.DailyRecord, 0, len(byDate))
	for _, r := range byDate {
		out = append(out, *r)
	}
	slices.SortFunc(out, func(a, b weather.DailyRecord) int { return a.Date.Compare(b.Date) })
	return out, nil
}

// ParseHourlyObservations parses an hourly observation timevaluepair response
// carrying TA_PT1H_AVG into one record per hour, skipping missing values.
func ParseHourlyObservations(data []byte) ([]weather.HourlyRecord, error) {
	var fc featureCollection
	if err := xml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("unmarshal WFS hourly observations: %w", err)
	}

	var out []weather.HourlyRecord
	for _, m := range fc.Members {
		if !strings.EqualFold(extractParam(m.Observation.ObservedProperty.Href), "TA_PT1H_AVG") {
			continue
		}
		for _, pt := range m.Observation.Result.TimeSeries.Points {
			t, err := time.Parse(time.RFC3339, pt.TVP.Time)
			if err != nil {
				continue
			}
			val := parseFloat(pt.TVP.Value)
			if val == nil || math.IsNaN(*val) {
				continue
			}
			out = append(out, weather.HourlyRecord{Time: t.UTC(), Temp: *val})
		}
	}
	slices.SortFunc(out, func(a, b weather.HourlyRecord) int { return a.Time.Compare(b.Time) })
	return out, nil
}
