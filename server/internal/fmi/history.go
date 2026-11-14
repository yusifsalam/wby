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
	dailyObservationChunk   = 365 * 24 * time.Hour
	hourlyObservationChunk  = 31 * 24 * time.Hour
	instantObservationChunk = 168 * time.Hour
	historyConcurrency      = 4
)

// FetchDailyObservations returns the daily mean/max/min temperature,
// precipitation and snow depth for a station between start and end (inclusive).
func (c *Client) FetchDailyObservations(ctx context.Context, fmisid int, start, end time.Time) ([]weather.DailyRecord, error) {
	chunks, err := c.fetchChunked(ctx, "fmi::observations::weather::daily::timevaluepair", "tday,tmax,tmin,rrday,snow", stationSelector(fmisid), start, end, dailyObservationChunk)
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

// FetchHourlyObservations returns the hourly mean temperature, humidity and
// wind for a station between start and end (inclusive).
func (c *Client) FetchHourlyObservations(ctx context.Context, fmisid int, start, end time.Time) ([]weather.HourlyRecord, error) {
	chunks, err := c.fetchChunked(ctx, "fmi::observations::weather::hourly::timevaluepair", "TA_PT1H_AVG,RH_PT1H_AVG,WS_PT1H_AVG,WG_PT1H_MAX", stationSelector(fmisid), start, end, hourlyObservationChunk)
	if err != nil {
		return nil, fmt.Errorf("fetch hourly observations: %w", err)
	}
	return mergeHourlyChunks(chunks)
}

// FetchInstantHourlyObservations returns on-the-hour temperature, humidity
// and 10-minute mean wind from the raw observation query, for stations the
// hourly product covers only briefly. Gusts are left out: a 10-minute gust
// at the hour understates the hourly maximum.
func (c *Client) FetchInstantHourlyObservations(ctx context.Context, fmisid int, start, end time.Time) ([]weather.HourlyRecord, error) {
	selector := stationSelector(fmisid)
	selector.Set("timestep", "60")
	chunks, err := c.fetchChunked(ctx, "fmi::observations::weather::timevaluepair", "temperature,rh,ws_10min", selector, start, end, instantObservationChunk)
	if err != nil {
		return nil, fmt.Errorf("fetch instant hourly observations: %w", err)
	}
	return mergeHourlyChunks(chunks)
}

func mergeHourlyChunks(chunks [][]byte) ([]weather.HourlyRecord, error) {
	byTime := make(map[time.Time]weather.HourlyRecord)
	for _, data := range chunks {
		recs, err := ParseHourlyObservations(data)
		if err != nil {
			return nil, err
		}
		for _, r := range recs {
			byTime[r.Time] = r
		}
	}
	out := make([]weather.HourlyRecord, 0, len(byTime))
	for _, r := range byTime {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b weather.HourlyRecord) int { return a.Time.Compare(b.Time) })
	return out, nil
}

// Half-widths of the station search box for precipitation gauges, in degrees:
// roughly 45 km each way at Finnish latitudes.
const (
	precipSearchHalfLat = 0.4
	precipSearchHalfLon = 0.8
)

// FetchHourlyPrecipitationNear returns the hourly precipitation accumulation
// between start and end (inclusive) from the gauge picked by
// pickPrecipitationGauge; each record's PrecipMm covers the hour ending at its
// time. Not every station has a gauge, so the nearest weather station is not
// necessarily the one returned. ErrNoStation when none report.
func (c *Client) FetchHourlyPrecipitationNear(ctx context.Context, lat, lon float64, start, end time.Time) (weather.PrecipitationObservations, error) {
	bbox := fmt.Sprintf("%.3f,%.3f,%.3f,%.3f", lon-precipSearchHalfLon, lat-precipSearchHalfLat, lon+precipSearchHalfLon, lat+precipSearchHalfLat)
	chunks, err := c.fetchChunked(ctx, "fmi::observations::weather::hourly::timevaluepair", "PRA_PT1H_ACC", url.Values{"bbox": {bbox}}, start, end, hourlyObservationChunk)
	if err != nil {
		return weather.PrecipitationObservations{}, fmt.Errorf("fetch hourly precipitation: %w", err)
	}
	byStation := make(map[int]*weather.PrecipitationObservations)
	for _, data := range chunks {
		stations, err := parseHourlyObservationsByStation(data)
		if err != nil {
			return weather.PrecipitationObservations{}, err
		}
		for _, st := range stations {
			if !validStation(st.Station) {
				continue
			}
			var recs []weather.HourlyRecord
			for _, r := range st.Hourly {
				if r.PrecipMm != nil {
					recs = append(recs, r)
				}
			}
			if len(recs) == 0 {
				continue
			}
			agg, ok := byStation[st.Station.FMISID]
			if !ok {
				agg = &weather.PrecipitationObservations{Station: st.Station, DistanceKM: haversineKm(lat, lon, st.Station.Lat, st.Station.Lon)}
				byStation[st.Station.FMISID] = agg
			}
			agg.Hourly = append(agg.Hourly, recs...)
		}
	}
	gauges := make([]*weather.PrecipitationObservations, 0, len(byStation))
	for _, st := range byStation {
		slices.SortFunc(st.Hourly, func(a, b weather.HourlyRecord) int { return a.Time.Compare(b.Time) })
		st.Hourly = slices.CompactFunc(st.Hourly, func(a, b weather.HourlyRecord) bool { return a.Time.Equal(b.Time) })
		gauges = append(gauges, st)
	}
	best := pickPrecipitationGauge(gauges, start)
	if best == nil {
		return weather.PrecipitationObservations{}, weather.ErrNoStation
	}
	return *best, nil
}

// minGaugeCoverage is the share of hours a gauge must report to be preferred
// over a more complete one farther away.
const minGaugeCoverage = 0.9

// pickPrecipitationGauge returns the nearest gauge that reported at least
// minGaugeCoverage of the hours between start and the newest record of any
// gauge. When none did, it returns the one with the most hours. Each gauge's
// Hourly must be sorted and free of duplicates.
func pickPrecipitationGauge(gauges []*weather.PrecipitationObservations, start time.Time) *weather.PrecipitationObservations {
	var latest time.Time
	for _, g := range gauges {
		if n := len(g.Hourly); n > 0 && g.Hourly[n-1].Time.After(latest) {
			latest = g.Hourly[n-1].Time
		}
	}
	required := int(math.Ceil(minGaugeCoverage * float64(latest.Sub(start)/time.Hour)))

	hours := func(g *weather.PrecipitationObservations) int {
		n := 0
		for _, r := range g.Hourly {
			if r.Time.After(start) {
				n++
			}
		}
		return n
	}
	nearer := func(a, b *weather.PrecipitationObservations) bool {
		if a.DistanceKM != b.DistanceKM {
			return a.DistanceKM < b.DistanceKM
		}
		return a.Station.FMISID < b.Station.FMISID
	}

	var best *weather.PrecipitationObservations
	bestHours, bestComplete := 0, false
	for _, g := range gauges {
		n := hours(g)
		complete := n >= required
		var better bool
		switch {
		case best == nil:
			better = true
		case complete != bestComplete:
			better = complete
		case complete || n == bestHours:
			better = nearer(g, best)
		default:
			better = n > bestHours
		}
		if better {
			best, bestHours, bestComplete = g, n, complete
		}
	}
	return best
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0
	toRad := func(deg float64) float64 { return deg * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(a))
}

// fetchChunked splits [start, end] into chunk-sized WFS requests, run a few
// at a time. selector picks the stations (fmisid or bbox).
func (c *Client) fetchChunked(ctx context.Context, query, parameters string, selector url.Values, start, end time.Time, chunk time.Duration) ([][]byte, error) {
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
				"parameters":     {parameters},
				"starttime":      {sp.start.UTC().Format(time.RFC3339)},
				"endtime":        {sp.end.UTC().Format(time.RFC3339)},
			}
			for k, v := range selector {
				params[k] = v
			}
			results[i], errs[i] = c.fetchWithRetry(ctx, params)
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
			case "snow":
				r.SnowCm = val
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

func stationSelector(fmisid int) url.Values {
	return url.Values{"fmisid": {strconv.Itoa(fmisid)}}
}

// ParseHourlyObservations parses a single-station hourly observation
// timevaluepair response into one record per hour, leaving missing parameters
// nil. Hours with no values at all are dropped.
func ParseHourlyObservations(data []byte) ([]weather.HourlyRecord, error) {
	stations, err := parseHourlyObservationsByStation(data)
	if err != nil {
		return nil, err
	}
	var out []weather.HourlyRecord
	for _, st := range stations {
		out = append(out, st.Hourly...)
	}
	slices.SortFunc(out, func(a, b weather.HourlyRecord) int { return a.Time.Compare(b.Time) })
	return out, nil
}

// parseHourlyObservationsByStation parses an hourly observation timevaluepair
// response into per-station hourly records.
func parseHourlyObservationsByStation(data []byte) ([]weather.PrecipitationObservations, error) {
	var fc featureCollection
	if err := xml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("unmarshal WFS hourly observations: %w", err)
	}

	type stationHours struct {
		station weather.Station
		byTime  map[time.Time]*weather.HourlyRecord
	}
	stations := make(map[int]*stationHours)
	var order []int
	for _, m := range fc.Members {
		fmisid, name, lat, lon, wmo := extractStationInfo(m.Observation)
		st, ok := stations[fmisid]
		if !ok {
			st = &stationHours{
				station: weather.Station{FMISID: fmisid, Name: name, Lat: lat, Lon: lon, WMOCode: wmo},
				byTime:  make(map[time.Time]*weather.HourlyRecord),
			}
			stations[fmisid] = st
			order = append(order, fmisid)
		}
		param := strings.ToUpper(extractParam(m.Observation.ObservedProperty.Href))
		for _, pt := range m.Observation.Result.TimeSeries.Points {
			t, err := time.Parse(time.RFC3339, pt.TVP.Time)
			if err != nil {
				continue
			}
			val := parseFloat(pt.TVP.Value)
			if val == nil || math.IsNaN(*val) {
				continue
			}
			t = t.UTC()
			r, ok := st.byTime[t]
			if !ok {
				r = &weather.HourlyRecord{Time: t}
				st.byTime[t] = r
			}
			switch param {
			case "TA_PT1H_AVG", "TEMPERATURE":
				r.Temp = val
			case "RH_PT1H_AVG", "RH":
				r.Humidity = val
			case "WS_PT1H_AVG", "WS_10MIN":
				r.WindSpeed = val
			case "WG_PT1H_MAX":
				r.WindGust = val
			case "PRA_PT1H_ACC":
				r.PrecipMm = val
			}
		}
	}

	out := make([]weather.PrecipitationObservations, 0, len(order))
	for _, fmisid := range order {
		st := stations[fmisid]
		recs := make([]weather.HourlyRecord, 0, len(st.byTime))
		for _, r := range st.byTime {
			recs = append(recs, *r)
		}
		slices.SortFunc(recs, func(a, b weather.HourlyRecord) int { return a.Time.Compare(b.Time) })
		out = append(out, weather.PrecipitationObservations{Station: st.station, Hourly: recs})
	}
	return out, nil
}
