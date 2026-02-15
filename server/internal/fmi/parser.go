package fmi

import (
	"encoding/xml"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"wby/internal/weather"
)

// WFS XML types -- shared across observations and forecasts.
// Go's encoding/xml requires the full namespace URI for matching.

const (
	nsWFS    = "http://www.opengis.net/wfs/2.0"
	nsOM     = "http://www.opengis.net/om/2.0"
	nsOMSO   = "http://inspire.ec.europa.eu/schemas/omso/3.0"
	nsSAMS   = "http://www.opengis.net/samplingSpatial/2.0"
	nsSAM    = "http://www.opengis.net/sampling/2.0"
	nsGML    = "http://www.opengis.net/gml/3.2"
	nsWML2   = "http://www.opengis.net/waterml/2.0"
	nsTarget = "http://xml.fmi.fi/namespace/om/atmosphericfeatures/1.1"
	nsXLink  = "http://www.w3.org/1999/xlink"
)

type featureCollection struct {
	XMLName xml.Name `xml:"FeatureCollection"`
	Members []member `xml:"member"`
}

type member struct {
	Observation pointTimeSeries `xml:"PointTimeSeriesObservation"`
}

type pointTimeSeries struct {
	ObservedProperty observedProperty `xml:"observedProperty"`
	FeatureOfInterest featureOfInterest `xml:"featureOfInterest"`
	Result           tsResult          `xml:"result"`
}

type observedProperty struct {
	Href string `xml:"http://www.w3.org/1999/xlink href,attr"`
}

type featureOfInterest struct {
	Feature spatialFeature `xml:"SF_SpatialSamplingFeature"`
}

type spatialFeature struct {
	SampledFeature sampledFeature `xml:"sampledFeature"`
	Shape          shape          `xml:"shape"`
}

type sampledFeature struct {
	LocationCollection locationCollection `xml:"LocationCollection"`
}

type locationCollection struct {
	Members []locationMember `xml:"member"`
}

type locationMember struct {
	Location location `xml:"Location"`
}

type location struct {
	Identifier string    `xml:"identifier"`
	Names      []gmlName `xml:"name"`
}

type gmlName struct {
	CodeSpace string `xml:"codeSpace,attr"`
	Value     string `xml:",chardata"`
}

type shape struct {
	Point      gmlPoint   `xml:"Point"`
	MultiPoint multiPoint `xml:"MultiPoint"`
}

type gmlPoint struct {
	Pos string `xml:"pos"`
}

type multiPoint struct {
	Points []gmlPoint `xml:"pointMembers>Point"`
}

type tsResult struct {
	TimeSeries measurementTimeSeries `xml:"MeasurementTimeseries"`
}

type measurementTimeSeries struct {
	Points []measurementPoint `xml:"point"`
}

type measurementPoint struct {
	TVP timeValuePair `xml:"MeasurementTVP"`
}

type timeValuePair struct {
	Time  string `xml:"time"`
	Value string `xml:"value"`
}

// ObservationResult holds parsed observation data from FMI.
type ObservationResult struct {
	Stations     []weather.Station
	Observations []weather.Observation
}

// ParseObservations parses an FMI WFS observation response.
func ParseObservations(data []byte) (*ObservationResult, error) {
	var fc featureCollection
	if err := xml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("unmarshal WFS: %w", err)
	}

	if len(fc.Members) == 0 {
		return &ObservationResult{}, nil
	}

	stationMap := make(map[int]*weather.Station)
	type obsKey struct {
		fmisid int
		t      time.Time
	}
	obsMap := make(map[obsKey]*weather.Observation)

	for _, m := range fc.Members {
		param := extractParam(m.Observation.ObservedProperty.Href)
		fmisid, name, lat, lon, wmo := extractStationInfo(m.Observation)

		if _, ok := stationMap[fmisid]; !ok {
			stationMap[fmisid] = &weather.Station{
				FMISID:  fmisid,
				Name:    name,
				Lat:     lat,
				Lon:     lon,
				WMOCode: wmo,
			}
		}

		for _, pt := range m.Observation.Result.TimeSeries.Points {
			t, err := time.Parse(time.RFC3339, pt.TVP.Time)
			if err != nil {
				continue
			}
			val := parseFloat(pt.TVP.Value)

			key := obsKey{fmisid: fmisid, t: t}
			obs, ok := obsMap[key]
			if !ok {
				obs = &weather.Observation{FMISID: fmisid, ObservedAt: t}
				obsMap[key] = obs
			}

			switch param {
			case "temperature":
				obs.Temperature = val
			case "windspeedms":
				obs.WindSpeed = val
			case "winddirection":
				obs.WindDir = val
			case "humidity":
				obs.Humidity = val
			case "pressure":
				obs.Pressure = val
			}
		}
	}

	result := &ObservationResult{}
	for _, s := range stationMap {
		result.Stations = append(result.Stations, *s)
	}
	for _, o := range obsMap {
		result.Observations = append(result.Observations, *o)
	}

	// Sort for deterministic output: stations by FMISID, observations by time.
	slices.SortFunc(result.Stations, func(a, b weather.Station) int {
		return a.FMISID - b.FMISID
	})
	slices.SortFunc(result.Observations, func(a, b weather.Observation) int {
		return a.ObservedAt.Compare(b.ObservedAt)
	})

	return result, nil
}

// ParseForecast parses an FMI WFS Harmonie forecast response and aggregates
// hourly values into daily forecasts (high, low, avg wind, total precip).
func ParseForecast(data []byte, gridLat, gridLon float64) ([]weather.DailyForecast, error) {
	var fc featureCollection
	if err := xml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("unmarshal WFS forecast: %w", err)
	}

	type hourlyEntry struct {
		t   time.Time
		val float64
	}
	params := make(map[string][]hourlyEntry)

	for _, m := range fc.Members {
		param := extractParam(m.Observation.ObservedProperty.Href)
		for _, pt := range m.Observation.Result.TimeSeries.Points {
			t, err := time.Parse(time.RFC3339, pt.TVP.Time)
			if err != nil {
				continue
			}
			val := parseFloat(pt.TVP.Value)
			if val == nil {
				continue
			}
			params[param] = append(params[param], hourlyEntry{t: t, val: *val})
		}
	}

	type dayBucket struct {
		temps   []float64
		winds   []float64
		precip  float64
		symbols []string
	}
	days := make(map[string]*dayBucket)
	dayOrder := []string{}

	addDay := func(dateKey string) *dayBucket {
		if _, ok := days[dateKey]; !ok {
			days[dateKey] = &dayBucket{}
			dayOrder = append(dayOrder, dateKey)
		}
		return days[dateKey]
	}

	for _, e := range params["temperature"] {
		dk := e.t.Format("2006-01-02")
		b := addDay(dk)
		b.temps = append(b.temps, e.val)
	}
	for _, e := range params["windspeedms"] {
		dk := e.t.Format("2006-01-02")
		b := addDay(dk)
		b.winds = append(b.winds, e.val)
	}
	for _, e := range params["precipitation1h"] {
		dk := e.t.Format("2006-01-02")
		b := addDay(dk)
		b.precip += e.val
	}
	for _, e := range params["weathersymbol3"] {
		dk := e.t.Format("2006-01-02")
		b := addDay(dk)
		b.symbols = append(b.symbols, strconv.Itoa(int(e.val)))
	}

	now := time.Now()
	var forecasts []weather.DailyForecast
	for _, dk := range dayOrder {
		b := days[dk]
		date, _ := time.Parse("2006-01-02", dk)
		f := weather.DailyForecast{
			GridLat:   gridLat,
			GridLon:   gridLon,
			Date:      date,
			FetchedAt: now,
		}
		if len(b.temps) > 0 {
			hi, lo := b.temps[0], b.temps[0]
			for _, t := range b.temps[1:] {
				if t > hi {
					hi = t
				}
				if t < lo {
					lo = t
				}
			}
			f.TempHigh = &hi
			f.TempLow = &lo
		}
		if len(b.winds) > 0 {
			avg := 0.0
			for _, w := range b.winds {
				avg += w
			}
			avg /= float64(len(b.winds))
			f.WindSpeed = &avg
		}
		precip := b.precip
		f.PrecipMM = &precip
		if len(b.symbols) > 0 {
			f.Symbol = &b.symbols[len(b.symbols)/2]
		}
		forecasts = append(forecasts, f)
	}
	return forecasts, nil
}

func extractParam(href string) string {
	for _, part := range strings.Split(href, "&") {
		if strings.HasPrefix(part, "param=") {
			return strings.TrimPrefix(part, "param=")
		}
	}
	parts := strings.Split(href, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func extractStationInfo(pts pointTimeSeries) (fmisid int, name string, lat, lon float64, wmo string) {
	foi := pts.FeatureOfInterest.Feature
	for _, lm := range foi.SampledFeature.LocationCollection.Members {
		loc := lm.Location
		fmisid, _ = strconv.Atoi(loc.Identifier)
		for _, n := range loc.Names {
			switch {
			case strings.Contains(n.CodeSpace, "name"):
				name = n.Value
			case strings.Contains(n.CodeSpace, "wmo"):
				wmo = n.Value
			}
		}
	}

	pos := foi.Shape.Point.Pos
	if pos == "" && len(foi.Shape.MultiPoint.Points) > 0 {
		pos = foi.Shape.MultiPoint.Points[0].Pos
	}
	lat, lon = parsePos(pos)
	return
}

func parsePos(pos string) (float64, float64) {
	parts := strings.Fields(pos)
	if len(parts) != 2 {
		return 0, 0
	}
	lat, _ := strconv.ParseFloat(parts[0], 64)
	lon, _ := strconv.ParseFloat(parts[1], 64)
	return lat, lon
}

func parseFloat(s string) *float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) {
		return nil
	}
	return &v
}
