package weather

import (
	"math"
	"strconv"
	"time"
)

const DefaultPlaceTimezone = "Europe/Helsinki"

type Station struct {
	FMISID  int
	Name    string
	Lat     float64
	Lon     float64
	WMOCode string
}

type StationDistance struct {
	Station
	DistanceKM float64
}

type Observation struct {
	FMISID             int
	ObservedAt         time.Time
	Temperature        *float64
	WindSpeed          *float64
	WindGust           *float64
	WindDir            *float64
	Humidity           *float64
	DewPoint           *float64
	Pressure           *float64
	Precip1h           *float64
	PrecipIntensity    *float64
	SnowDepth          *float64
	Visibility         *float64
	TotalCloudCover    *float64
	WeatherCode        *float64
	ExtraNumericParams map[string]float64
}

type DailyForecast struct {
	GridLat                        float64
	GridLon                        float64
	Date                           time.Time
	FetchedAt                      time.Time
	TempHigh                       *float64
	TempLow                        *float64
	TempAvg                        *float64
	WindSpeed                      *float64
	WindDir                        *float64
	HumidityAvg                    *float64
	PrecipMM                       *float64
	Precip1hSum                    *float64
	Symbol                         *string
	DewPointAvg                    *float64
	FogIntensityAvg                *float64
	FrostProbabilityAvg            *float64
	SevereFrostProbabilityAvg      *float64
	GeopHeightAvg                  *float64
	PressureAvg                    *float64
	HighCloudCoverAvg              *float64
	LowCloudCoverAvg               *float64
	MediumCloudCoverAvg            *float64
	MiddleAndLowCloudCoverAvg      *float64
	TotalCloudCoverAvg             *float64
	HourlyMaximumGustMax           *float64
	HourlyMaximumWindSpeedMax      *float64
	PoPAvg                         *float64
	ProbabilityThunderstormAvg     *float64
	PotentialPrecipitationFormMode *float64
	PotentialPrecipitationTypeMode *float64
	PrecipitationFormMode          *float64
	PrecipitationTypeMode          *float64
	RadiationGlobalAvg             *float64
	RadiationLWAvg                 *float64
	WeatherNumberMode              *float64
	WeatherSymbol3Mode             *float64
	WindUMSAvg                     *float64
	WindVMSAvg                     *float64
	WindVectorMSAvg                *float64
	UVIndexAvg                     *float64
}

type HourlyForecast struct {
	Time        time.Time
	FetchedAt   time.Time
	Temperature *float64
	FeelsLike   *float64 // FMI FeelsLike (apparent temperature), °C
	WindSpeed   *float64
	WindDir     *float64
	Humidity    *float64
	Precip1h    *float64
	Symbol      *string
	UVCumulated *float64
	WindGust    *float64 // HourlyMaximumGust, m/s
	Pressure    *float64 // hPa
	CloudCover  *float64 // TotalCloudCover, %
	PoP         *float64 // probability of precipitation, %
}

type UVDataPoint struct {
	Time        time.Time
	UVCumulated float64
}

type CurrentWeather struct {
	Station     Station
	DistanceKM  float64
	Observation Observation
}

type WeatherResponse struct {
	Current  CurrentWeather
	Hourly   []HourlyForecast
	Forecast []DailyForecast
	Timezone string
}

type ForecastData struct {
	Forecasts []DailyForecast
	Timezone  string
}

type MapOverlayRequest struct {
	MinLon float64
	MinLat float64
	MaxLon float64
	MaxLat float64
	Width  int
	Height int
}

type TemperatureSample struct {
	Lat         float64
	Lon         float64
	Temperature float64
	ObservedAt  time.Time
}

// FieldSample is a single gridpoint of a GRIB field in its consumer-facing
// units (Celsius for temperature, mm/h for precipitation rate). It is the
// units-agnostic carrier the grib client emits; typed views like
// TemperatureSample are mapped from it.
type FieldSample struct {
	Lat        float64
	Lon        float64
	Value      float64
	ObservedAt time.Time
}

// FieldGrid is a regular lat/lon raster of a GRIB field in consumer units. It
// preserves the grid topology (unlike FieldSample) so clients can upload it as
// a texture and interpolate with hardware bilinear instead of point IDW.
//
// Values is row-major, north-to-south (row 0 = MaxLat), west-to-east, length
// Rows*Cols; a nil entry is a masked/missing cell. Min/Max bounds are the actual
// extents of the (possibly strided) grid, i.e. the centres of the corner cells.
type FieldGrid struct {
	Rows       int
	Cols       int
	MinLat     float64
	MaxLat     float64
	MinLon     float64
	MaxLon     float64
	Values     []float32 // row-major, NaN = masked
	ObservedAt time.Time
}

// GridRange returns the min and max over the grid's valid (non-NaN) cells, or
// (0, 0) when the grid is entirely masked.
func (g *FieldGrid) GridRange() (float64, float64) {
	minV, maxV := float32(math.Inf(1)), float32(math.Inf(-1))
	for _, v := range g.Values {
		if v != v {
			continue
		}
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	if math.IsInf(float64(minV), 1) {
		return 0, 0
	}
	return widen(minV), widen(maxV)
}

// widen converts a float32 to the float64 with the same shortest decimal
// representation (3.2 rather than 3.200000047683716).
func widen(v float32) float64 {
	f, _ := strconv.ParseFloat(strconv.FormatFloat(float64(v), 'g', -1, 32), 64)
	return f
}

type TemperatureOverlay struct {
	PNG      []byte
	DataTime time.Time
	MinTemp  float64
	MaxTemp  float64
}

type TemperatureSamplesResponse struct {
	DataTime time.Time
	MinTemp  float64
	MaxTemp  float64
	Samples  []TemperatureSample
	// Grid is set when the field came from the dense GRIB raster; clients
	// prefer it (texture bilinear) over Samples. Nil for station fallbacks.
	Grid *FieldGrid
}

type ClimateNormal struct {
	FMISID   int
	Month    int
	Period   string
	TempAvg  *float64
	TempHigh *float64
	TempLow  *float64
	PrecipMm *float64
}

type DailyClimateNormal struct {
	FMISID        int
	Period        string
	Month         int
	Day           int
	TempAvg       *float64
	TempHigh      *float64
	TempLow       *float64
	FeelsLikeAvg  *float64
	FeelsLikeHigh *float64
	FeelsLikeLow  *float64
	WindAvg       *float64 // m/s
	WindGust      *float64 // mean of the daily maximum gust, m/s
	HumidityAvg   *float64 // %
	PrecipMm      *float64 // mm/day
	PrecipDaysPct *float64 // share of days with >= 0.1 mm, %
	SnowCm        *float64
	// The hourly curves hold one mean per UTC hour (24 entries); nil when
	// the station lacks an hourly record for the period. TempHourlyP10 and
	// TempHourlyP90 bound the typical range for each hour.
	TempHourly      []float64
	TempHourlyP10   []float64
	TempHourlyP90   []float64
	FeelsLikeHourly []float64
	WindHourly      []float64
	HumidityHourly  []float64
	// DailyYears and HourlyYears are the station's record length within the
	// period in year-equivalents: days with a daily mean / 365.25 and hours
	// with a temperature / 8766.
	DailyYears  float64
	HourlyYears float64
}

// DailyNormalsCandidate is a station with daily normals for one calendar day.
type DailyNormalsCandidate struct {
	StationDistance
	DailyYears  float64
	HourlyYears float64
	HasTemp     bool
	HasHourly   bool
}

type DailyRecord struct {
	Date     time.Time
	TempAvg  *float64
	TempHigh *float64
	TempLow  *float64
	PrecipMm *float64
	SnowCm   *float64
}

type HourlyRecord struct {
	Time      time.Time
	Temp      *float64
	Humidity  *float64
	WindSpeed *float64
	WindGust  *float64
	// PrecipMm is the accumulation over the hour ending at Time.
	PrecipMm *float64
}

// PrecipitationObservations is a gauge station's hourly precipitation history
// with its distance from the requested point.
type PrecipitationObservations struct {
	Station    Station
	DistanceKM float64
	Hourly     []HourlyRecord
}

// PrecipitationToDate compares observed precipitation for today and the
// current month so far against the daily normals summed over the same days.
// Observed values come from the nearest gauge, which may differ from the
// normals station.
type PrecipitationToDate struct {
	Station               *Station
	StationDistanceKM     float64
	TodayObservedMm       *float64
	TodayNormalMm         *float64
	MonthToDateObservedMm *float64
	MonthToDateNormalMm   *float64
	MonthNormalMm         *float64
	ObservedThrough       *time.Time
}

type DailyNormalsResult struct {
	Station    Station
	DistanceKM float64
	// HourlyStation supplies the hourly-derived fields when Station has no
	// hourly record; nil when they come from Station itself.
	HourlyStation      *Station
	HourlyDistanceKM   float64
	Period             string
	Today              DailyClimateNormal
	TempNowNormal      *float64
	TempDiff           *float64
	FeelsLikeNowNormal *float64
	WindNowNormal      *float64
	HumidityNowNormal  *float64
	Precipitation      PrecipitationToDate
	Daily              []DailyClimateNormal
}

type InterpolatedNormal struct {
	TempAvg     *float64
	TempHigh    *float64
	TempLow     *float64
	PrecipMmDay *float64
	TempDiff    *float64
}

type LeaderboardEntry struct {
	StatType    string
	StationName string
	Lat         float64
	Lon         float64
	Value       float64
	Unit        string
	DistanceKM  float64
	ObservedAt  time.Time
}
