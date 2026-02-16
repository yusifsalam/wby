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
	Name string `xml:"name"`
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
		param := strings.ToLower(extractParam(m.Observation.ObservedProperty.Href))
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
			case "temperature", "t2m":
				obs.Temperature = val
			case "windspeedms", "ws_10min":
				obs.WindSpeed = val
			case "windgust", "gustspeed", "maximumwind", "wg_10min":
				obs.WindGust = val
			case "winddirection", "wd_10min":
				obs.WindDir = val
			case "humidity", "rh":
				obs.Humidity = val
			case "dewpoint", "td":
				obs.DewPoint = val
			case "pressure", "p_sea":
				obs.Pressure = val
			case "precipitation1h", "precipitationamount", "r_1h":
				obs.Precip1h = val
			case "precipitationintensity", "ri_10min":
				obs.PrecipIntensity = val
			case "snowdepth", "snow_aws":
				obs.SnowDepth = val
			case "visibility", "vis":
				obs.Visibility = val
			case "totalcloudcover", "cloudcover", "n_man":
				obs.TotalCloudCover = val
			case "weather", "weathercode", "wawa":
				obs.WeatherCode = val
			default:
				if val != nil {
					if obs.ExtraNumericParams == nil {
						obs.ExtraNumericParams = make(map[string]float64)
					}
					obs.ExtraNumericParams[param] = *val
				}
			}
		}
	}

	result := &ObservationResult{}
	for _, s := range stationMap {
		result.Stations = append(result.Stations, *s)
	}
	for _, o := range obsMap {
		if !hasAnyValue(o) {
			continue
		}
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

// ParseForecast parses an FMI WFS forecast response and aggregates hourly
// values into daily forecast columns.
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
		param := strings.ToLower(extractParam(m.Observation.ObservedProperty.Href))
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
		values map[string][]float64
	}
	days := make(map[string]*dayBucket)
	dayOrder := []string{}

	addValue := func(dateKey, param string, value float64) {
		b, ok := days[dateKey]
		if !ok {
			b = &dayBucket{values: make(map[string][]float64)}
			days[dateKey] = b
			dayOrder = append(dayOrder, dateKey)
		}
		b.values[param] = append(b.values[param], value)
	}

	for param, entries := range params {
		for _, e := range entries {
			addValue(e.t.Format("2006-01-02"), param, e.val)
		}
	}

	now := time.Now()
	var forecasts []weather.DailyForecast
	for _, dk := range dayOrder {
		b := days[dk]
		date, _ := time.Parse("2006-01-02", dk)
		vals := func(param string) []float64 { return b.values[param] }

		f := weather.DailyForecast{
			GridLat:   gridLat,
			GridLon:   gridLon,
			Date:      date,
			FetchedAt: now,
		}
		tempVals := vals("temperature")
		if len(tempVals) > 0 {
			hi, lo := tempVals[0], tempVals[0]
			for _, t := range tempVals[1:] {
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
		f.TempAvg = avgPtr(tempVals)
		f.WindSpeed = avgPtr(vals("windspeedms"))
		f.WindDir = circularMeanDegreesPtr(vals("winddirection"))
		f.HumidityAvg = avgPtr(vals("humidity"))
		f.PrecipMM = sumPtr(vals("precipitation1h"))
		f.Precip1hSum = f.PrecipMM
		f.Symbol = modeRoundedStringPtr(vals("weathersymbol3"))

		f.DewPointAvg = avgPtr(vals("dewpoint"))
		f.FogIntensityAvg = avgPtr(vals("fogintensity"))
		f.FrostProbabilityAvg = avgPtr(vals("frostprobability"))
		f.SevereFrostProbabilityAvg = avgPtr(vals("severefrostprobability"))
		f.GeopHeightAvg = avgPtr(vals("geopheight"))
		f.PressureAvg = avgPtr(vals("pressure"))
		f.HighCloudCoverAvg = avgPtr(vals("highcloudcover"))
		f.LowCloudCoverAvg = avgPtr(vals("lowcloudcover"))
		f.MediumCloudCoverAvg = avgPtr(vals("mediumcloudcover"))
		f.MiddleAndLowCloudCoverAvg = avgPtr(vals("middleandlowcloudcover"))
		f.TotalCloudCoverAvg = avgPtr(vals("totalcloudcover"))
		f.HourlyMaximumGustMax = maxPtr(vals("hourlymaximumgust"))
		f.HourlyMaximumWindSpeedMax = maxPtr(vals("hourlymaximumwindspeed"))
		f.PoPAvg = avgPtr(vals("pop"))
		f.ProbabilityThunderstormAvg = avgPtr(vals("probabilitythunderstorm"))
		f.PotentialPrecipitationFormMode = modeRoundedFloatPtr(vals("potentialprecipitationform"))
		f.PotentialPrecipitationTypeMode = modeRoundedFloatPtr(vals("potentialprecipitationtype"))
		f.PrecipitationFormMode = modeRoundedFloatPtr(vals("precipitationform"))
		f.PrecipitationTypeMode = modeRoundedFloatPtr(vals("precipitationtype"))
		f.RadiationGlobalAvg = avgPtr(vals("radiationglobal"))
		f.RadiationLWAvg = avgPtr(vals("radiationlw"))
		f.WeatherNumberMode = modeRoundedFloatPtr(vals("weathernumber"))
		f.WeatherSymbol3Mode = modeRoundedFloatPtr(vals("weathersymbol3"))
		f.WindUMSAvg = avgPtr(vals("windums"))
		f.WindVMSAvg = avgPtr(vals("windvms"))
		f.WindVectorMSAvg = avgPtr(vals("windvectorms"))

		forecasts = append(forecasts, f)
	}
	return forecasts, nil
}

// ParseHourlyForecast parses hourly time/value pairs for temperature and weather symbol.
func ParseHourlyForecast(data []byte, limit int) ([]weather.HourlyForecast, error) {
	var fc featureCollection
	if err := xml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("unmarshal WFS hourly forecast: %w", err)
	}

	type hourlyPoint struct {
		t       time.Time
		temp    *float64
		wind    *float64
		windDir *float64
		rh      *float64
		precip  *float64
		sym     *string
	}
	byTime := make(map[time.Time]*hourlyPoint)

	for _, m := range fc.Members {
		param := strings.ToLower(extractParam(m.Observation.ObservedProperty.Href))
		for _, pt := range m.Observation.Result.TimeSeries.Points {
			t, err := time.Parse(time.RFC3339, pt.TVP.Time)
			if err != nil {
				continue
			}
			val := parseFloat(pt.TVP.Value)
			if val == nil {
				continue
			}

			p, ok := byTime[t]
			if !ok {
				p = &hourlyPoint{t: t}
				byTime[t] = p
			}

			switch param {
			case "temperature":
				p.temp = val
			case "windspeedms":
				p.wind = val
			case "winddirection":
				p.windDir = val
			case "humidity":
				p.rh = val
			case "precipitation1h":
				p.precip = val
			case "weathersymbol3":
				s := strconv.Itoa(int(math.Round(*val)))
				p.sym = &s
			}
		}
	}

	var items []hourlyPoint
	for _, p := range byTime {
		if p.temp == nil && p.wind == nil && p.windDir == nil && p.rh == nil && p.precip == nil && p.sym == nil {
			continue
		}
		items = append(items, *p)
	}
	slices.SortFunc(items, func(a, b hourlyPoint) int {
		return a.t.Compare(b.t)
	})

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	result := make([]weather.HourlyForecast, 0, len(items))
	for _, p := range items {
		result = append(result, weather.HourlyForecast{
			Time:        p.t,
			Temperature: p.temp,
			WindSpeed:   p.wind,
			WindDir:     p.windDir,
			Humidity:    p.rh,
			Precip1h:    p.precip,
			Symbol:      p.sym,
		})
	}
	return result, nil
}

func avgPtr(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	avg := sum / float64(len(values))
	return &avg
}

func sumPtr(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return &sum
}

func maxPtr(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	maxV := values[0]
	for _, v := range values[1:] {
		if v > maxV {
			maxV = v
		}
	}
	return &maxV
}

func modeRoundedFloatPtr(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	counts := make(map[int]int)
	bestKey := 0
	bestCount := 0
	for _, v := range values {
		key := int(math.Round(v))
		counts[key]++
		if counts[key] > bestCount || (counts[key] == bestCount && key < bestKey) {
			bestKey = key
			bestCount = counts[key]
		}
	}
	mode := float64(bestKey)
	return &mode
}

func modeRoundedStringPtr(values []float64) *string {
	mode := modeRoundedFloatPtr(values)
	if mode == nil {
		return nil
	}
	s := strconv.Itoa(int(math.Round(*mode)))
	return &s
}

func circularMeanDegreesPtr(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	var sinSum, cosSum float64
	for _, v := range values {
		rad := v * math.Pi / 180.0
		sinSum += math.Sin(rad)
		cosSum += math.Cos(rad)
	}
	if sinSum == 0 && cosSum == 0 {
		return nil
	}
	mean := math.Atan2(sinSum, cosSum) * 180.0 / math.Pi
	if mean < 0 {
		mean += 360.0
	}
	return &mean
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
		var fallbackName string
		for _, n := range loc.Names {
			value := strings.TrimSpace(n.Value)
			if value == "" {
				continue
			}
			switch {
			case isLocationNameCodeSpace(n.CodeSpace):
				if name == "" || isLikelyCodeValue(name) {
					name = value
				}
			case isLocationWMOCodeSpace(n.CodeSpace):
				wmo = value
				if fallbackName == "" {
					fallbackName = value
				}
			case fallbackName == "":
				fallbackName = value
			}
		}
		if name == "" {
			name = fallbackName
		}
	}

	pos := foi.Shape.Point.Pos
	if pos == "" && len(foi.Shape.MultiPoint.Points) > 0 {
		pos = foi.Shape.MultiPoint.Points[0].Pos
	}
	if name == "" {
		name = strings.TrimSpace(foi.Shape.Point.Name)
	}
	if name == "" && len(foi.Shape.MultiPoint.Points) > 0 {
		name = strings.TrimSpace(foi.Shape.MultiPoint.Points[0].Name)
	}
	if name == "" && wmo != "" {
		name = wmo
	}
	if name == "" {
		name = strconv.Itoa(fmisid)
	}
	lat, lon = parsePos(pos)
	return
}

func isLocationNameCodeSpace(codeSpace string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(codeSpace)), "/locationcode/name")
}

func isLocationWMOCodeSpace(codeSpace string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(codeSpace)), "/locationcode/wmo")
}

func isLikelyCodeValue(v string) bool {
	if v == "" {
		return false
	}
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
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

func hasAnyValue(o *weather.Observation) bool {
	return o.Temperature != nil || o.WindSpeed != nil || o.WindGust != nil ||
		o.WindDir != nil || o.Humidity != nil || o.DewPoint != nil ||
		o.Pressure != nil || o.Precip1h != nil || o.PrecipIntensity != nil ||
		o.SnowDepth != nil || o.Visibility != nil || o.TotalCloudCover != nil ||
		o.WeatherCode != nil || len(o.ExtraNumericParams) > 0
}
