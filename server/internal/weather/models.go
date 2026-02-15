package weather

import "time"

type Station struct {
	FMISID  int
	Name    string
	Lat     float64
	Lon     float64
	WMOCode string
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
	GridLat                         float64
	GridLon                         float64
	Date                            time.Time
	FetchedAt                       time.Time
	TempHigh                        *float64
	TempLow                         *float64
	TempAvg                         *float64
	WindSpeed                       *float64
	WindDir                         *float64
	HumidityAvg                     *float64
	PrecipMM                        *float64
	Precip1hSum                     *float64
	Symbol                          *string
	DewPointAvg                     *float64
	FogIntensityAvg                 *float64
	FrostProbabilityAvg             *float64
	SevereFrostProbabilityAvg       *float64
	GeopHeightAvg                   *float64
	PressureAvg                     *float64
	HighCloudCoverAvg               *float64
	LowCloudCoverAvg                *float64
	MediumCloudCoverAvg             *float64
	MiddleAndLowCloudCoverAvg       *float64
	TotalCloudCoverAvg              *float64
	HourlyMaximumGustMax            *float64
	HourlyMaximumWindSpeedMax       *float64
	PoPAvg                          *float64
	ProbabilityThunderstormAvg      *float64
	PotentialPrecipitationFormMode  *float64
	PotentialPrecipitationTypeMode  *float64
	PrecipitationFormMode           *float64
	PrecipitationTypeMode           *float64
	RadiationGlobalAvg              *float64
	RadiationLWAvg                  *float64
	WeatherNumberMode               *float64
	WeatherSymbol3Mode              *float64
	WindUMSAvg                      *float64
	WindVMSAvg                      *float64
	WindVectorMSAvg                 *float64
}

type HourlyForecast struct {
	Time        time.Time
	Temperature *float64
	WindSpeed   *float64
	WindDir     *float64
	Humidity    *float64
	Precip1h    *float64
	Symbol      *string
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
}
