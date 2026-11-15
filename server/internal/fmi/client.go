package fmi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"wby/internal/weather"
)

type Client struct {
	baseURL       string
	apiKey        string
	timeseriesURL string
	downloadURL   string
	httpClient    *http.Client
}

const forecastDays = 11
const hourlyForecastHours = weather.HourlyForecastHours

// forecastParameters is the explicit parameter list for the edited point
// forecast. Setting "parameters" replaces FMI's default set, so this must
// spell out every default parameter the parser relies on, plus SmartSymbol
// (the weather symbol used by the official FMI app; night codes are +100) and
// FeelsLike (FMI's apparent temperature: wind, humidity and radiation), which
// are not part of the default set.
const forecastParameters = "DewPoint,FogIntensity,FrostProbability,GeopHeight,HighCloudCover," +
	"HourlyMaximumGust,HourlyMaximumWindSpeed,Humidity,LowCloudCover,MediumCloudCover," +
	"MiddleAndLowCloudCover,PoP,PotentialPrecipitationForm,PotentialPrecipitationType," +
	"Precipitation1h,PrecipitationForm,PrecipitationType,Pressure,ProbabilityThunderstorm," +
	"RadiationGlobal,RadiationLW,SevereFrostProbability,Temperature,TotalCloudCover," +
	"WeatherNumber,WeatherSymbol3,WindDirection,WindSpeedMS,WindUMS,WindVectorMS,WindVMS," +
	"SmartSymbol,FeelsLike"

func NewClient(baseURL, apiKey, timeseriesURL string) *Client {
	return NewClientWithDownload(baseURL, apiKey, timeseriesURL, "")
}

func NewClientWithDownload(baseURL, apiKey, timeseriesURL, downloadURL string) *Client {
	return &Client{
		baseURL:       baseURL,
		apiKey:        apiKey,
		timeseriesURL: timeseriesURL,
		downloadURL:   downloadURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchObservations returns the 10-minute observations of every station from
// since up to now. A zero since leaves the start to FMI, which then answers
// with its default window of the last 12 hours.
func (c *Client) FetchObservations(ctx context.Context, since time.Time) (*ObservationResult, error) {
	params := url.Values{
		"service":        {"WFS"},
		"version":        {"2.0.0"},
		"request":        {"getFeature"},
		"storedquery_id": {"fmi::observations::weather::timevaluepair"},
		"timestep":       {"10"},
		"maxlocations":   {"200"},
		// FMI currently returns empty results without an explicit area filter.
		// This bbox covers Finland where the app data is sourced.
		"bbox": {"19,59,32,71"},
	}
	if !since.IsZero() {
		params.Set("starttime", since.UTC().Truncate(10*time.Minute).Format(time.RFC3339))
	}

	data, err := c.fetchWithRetry(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch observations: %w", err)
	}
	return ParseObservations(data)
}

func (c *Client) FetchForecast(ctx context.Context, lat, lon float64) (weather.ForecastData, error) {
	start, end := forecastTimeWindow(forecastDays)

	params := url.Values{
		"service":        {"WFS"},
		"version":        {"2.0.0"},
		"request":        {"getFeature"},
		"storedquery_id": {"fmi::forecast::edited::weather::scandinavia::point::timevaluepair"},
		"latlon":         {fmt.Sprintf("%f,%f", lat, lon)},
		"parameters":     {forecastParameters},
		"timestep":       {"60"},
		"starttime":      {start},
		"endtime":        {end},
	}

	data, err := c.fetch(ctx, params)
	if err != nil {
		return weather.ForecastData{}, fmt.Errorf("fetch forecast: %w", err)
	}
	return ParseForecast(data, lat, lon)
}

// FetchHourlyForecast returns up to `hours` hourly entries from `start`
// (truncated to the hour). FMI serves hours that have already passed from
// the same edited forecast, so `start` may lie earlier today.
func (c *Client) FetchHourlyForecast(ctx context.Context, lat, lon float64, start time.Time, hours int) ([]weather.HourlyForecast, error) {
	if hours <= 0 {
		hours = hourlyForecastHours
	}
	startTime := start.UTC().Truncate(time.Hour)
	endTime := startTime.Add(time.Duration(hours-1) * time.Hour)

	params := url.Values{
		"service":        {"WFS"},
		"version":        {"2.0.0"},
		"request":        {"getFeature"},
		"storedquery_id": {"fmi::forecast::edited::weather::scandinavia::point::timevaluepair"},
		"latlon":         {fmt.Sprintf("%f,%f", lat, lon)},
		"parameters":     {forecastParameters},
		"timestep":       {"60"},
		"starttime":      {startTime.Format(time.RFC3339)},
		"endtime":        {endTime.Format(time.RFC3339)},
	}

	data, err := c.fetch(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch hourly forecast: %w", err)
	}
	return ParseHourlyForecast(data, hours)
}

// FetchUVForecast returns hourly UV index points from start onwards. FMI's uv
// producer covers today (from 00:00 UTC) and the following day, so two days of
// steps from the local midnight span everything it has.
func (c *Client) FetchUVForecast(ctx context.Context, lat, lon float64, start time.Time) ([]weather.UVDataPoint, error) {
	if c.apiKey == "" {
		return nil, nil
	}

	startTime := start.UTC().Truncate(time.Hour).Format(time.RFC3339)
	reqURL := fmt.Sprintf(
		"%s/fmi-apikey/%s/timeseries?param=epochtime,uvCumulated&producer=uv&format=json&latlon=%f,%f&timesteps=48&starttime=%s",
		c.timeseriesURL, c.apiKey, lat, lon, startTime,
	)

	body, err := c.getBody(ctx, "UV API", reqURL, maxUVBody)
	if err != nil {
		return nil, fmt.Errorf("fetch UV forecast: %w", err)
	}

	var raw []struct {
		EpochTime   int64    `json:"epochtime"`
		UVCumulated *float64 `json:"uvCumulated"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse UV response (%d bytes): %w", len(body), err)
	}

	var points []weather.UVDataPoint
	for _, r := range raw {
		if r.UVCumulated == nil {
			continue
		}
		points = append(points, weather.UVDataPoint{
			Time:        time.Unix(r.EpochTime, 0).UTC(),
			UVCumulated: *r.UVCumulated,
		})
	}
	return points, nil
}

// forecastTimeWindow spans `days` whole local days: from today's midnight (so
// today's aggregates cover the elapsed hours too) through 23:00 on the last
// day. The app covers Finland only, so the place timezone is known before the
// response names it.
func forecastTimeWindow(days int) (start, end string) {
	if days < 1 {
		days = 1
	}
	loc, err := time.LoadLocation(weather.DefaultPlaceTimezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	startTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endTime := startTime.AddDate(0, 0, days).Add(-time.Hour)
	return startTime.UTC().Format(time.RFC3339), endTime.UTC().Format(time.RFC3339)
}

func (c *Client) FetchClimateNormals(ctx context.Context, fmisids string) ([]byte, error) {
	params := url.Values{
		"service":        {"WFS"},
		"version":        {"2.0.0"},
		"request":        {"getFeature"},
		"storedquery_id": {"fmi::observations::weather::monthly::30year::timevaluepair"},
		"fmisid":         {fmisids},
		"starttime":      {"1991-01-01T00:00:00Z"},
	}
	return c.fetch(ctx, params)
}

func (c *Client) fetch(ctx context.Context, params url.Values) ([]byte, error) {
	return c.getBody(ctx, "FMI", c.baseURL+"?"+params.Encode(), maxWFSBody)
}

// fetchWithRetry is fetch repeated as Retry allows.
func (c *Client) fetchWithRetry(ctx context.Context, params url.Values) ([]byte, error) {
	var data []byte
	err := Retry(ctx, func() (err error) {
		data, err = c.fetch(ctx, params)
		return err
	})
	return data, err
}
