package fmi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"wby/internal/weather"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

const forecastDays = 11
const hourlyForecastHours = 12

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) FetchObservations(ctx context.Context) (*ObservationResult, error) {
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

	data, err := c.fetch(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch observations: %w", err)
	}
	return ParseObservations(data)
}

func (c *Client) FetchForecast(ctx context.Context, lat, lon float64) ([]weather.DailyForecast, error) {
	start, end := forecastTimeWindowUTC(forecastDays)

	params := url.Values{
		"service":        {"WFS"},
		"version":        {"2.0.0"},
		"request":        {"getFeature"},
		"storedquery_id": {"fmi::forecast::edited::weather::scandinavia::point::timevaluepair"},
		"latlon":         {fmt.Sprintf("%f,%f", lat, lon)},
		"timestep":       {"60"},
		"starttime":      {start},
		"endtime":        {end},
	}

	data, err := c.fetch(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch forecast: %w", err)
	}
	return ParseForecast(data, lat, lon)
}

func (c *Client) FetchHourlyForecast(ctx context.Context, lat, lon float64, limit int) ([]weather.HourlyForecast, error) {
	hours := limit
	if hours <= 0 {
		hours = hourlyForecastHours
	}
	start, end := forecastHoursWindowUTC(hours)

	params := url.Values{
		"service":        {"WFS"},
		"version":        {"2.0.0"},
		"request":        {"getFeature"},
		"storedquery_id": {"fmi::forecast::edited::weather::scandinavia::point::timevaluepair"},
		"latlon":         {fmt.Sprintf("%f,%f", lat, lon)},
		"timestep":       {"60"},
		"starttime":      {start},
		"endtime":        {end},
	}

	data, err := c.fetch(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch hourly forecast: %w", err)
	}
	return ParseHourlyForecast(data, hours)
}

func forecastTimeWindowUTC(days int) (start, end string) {
	if days < 1 {
		days = 1
	}
	startTime := time.Now().UTC().Truncate(time.Hour)
	// Inclusive window: today + next (days-1) days.
	endTime := startTime.AddDate(0, 0, days-1)
	return startTime.Format(time.RFC3339), endTime.Format(time.RFC3339)
}

func forecastHoursWindowUTC(hours int) (start, end string) {
	if hours < 1 {
		hours = 1
	}
	startTime := time.Now().UTC().Truncate(time.Hour)
	// Inclusive window: current hour + next (hours-1) hours.
	endTime := startTime.Add(time.Duration(hours-1) * time.Hour)
	return startTime.Format(time.RFC3339), endTime.Format(time.RFC3339)
}

func (c *Client) fetch(ctx context.Context, params url.Values) ([]byte, error) {
	reqURL := c.baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("FMI returned %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}
