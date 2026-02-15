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
	FMISID      int
	ObservedAt  time.Time
	Temperature *float64
	WindSpeed   *float64
	WindDir     *float64
	Humidity    *float64
	Pressure    *float64
}

type DailyForecast struct {
	GridLat   float64
	GridLon   float64
	Date      time.Time
	FetchedAt time.Time
	TempHigh  *float64
	TempLow   *float64
	WindSpeed *float64
	PrecipMM  *float64
	Symbol    *string
}

type CurrentWeather struct {
	Station     Station
	DistanceKM  float64
	Observation Observation
}

type WeatherResponse struct {
	Current  CurrentWeather
	Forecast []DailyForecast
}
