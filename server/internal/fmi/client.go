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
		"parameters":     {"temperature,windspeedms,winddirection,humidity,pressure"},
		"timestep":       {"10"},
		"maxlocations":   {"200"},
	}

	data, err := c.fetch(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch observations: %w", err)
	}
	return ParseObservations(data)
}

func (c *Client) FetchForecast(ctx context.Context, lat, lon float64) ([]weather.DailyForecast, error) {
	params := url.Values{
		"service":        {"WFS"},
		"version":        {"2.0.0"},
		"request":        {"getFeature"},
		"storedquery_id": {"fmi::forecast::harmonie::surface::point::timevaluepair"},
		"latlon":         {fmt.Sprintf("%f,%f", lat, lon)},
		"parameters":     {"temperature,windspeedms,winddirection,humidity,precipitation1h,weathersymbol3"},
		"timestep":       {"60"},
	}

	data, err := c.fetch(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch forecast: %w", err)
	}
	return ParseForecast(data, lat, lon)
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
