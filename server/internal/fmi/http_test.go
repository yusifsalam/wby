package fmi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"wby/internal/weather"
)

const testAPIKey = "sekrit-api-key"

func fastRetries(t *testing.T) {
	t.Helper()
	saved := retryBackoff
	retryBackoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { retryBackoff = saved })
}

func wmsTileRequest() weather.WMSTileRequest {
	return weather.WMSTileRequest{Layer: "obs", Time: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC),
		MinLon: 19, MinLat: 59, MaxLon: 32, MaxLat: 71, Width: 64, Height: 64}
}

func TestTransportErrorsDoNotCarryTheAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	unreachable := srv.URL
	srv.Close()
	c := NewClientWithWMS(unreachable, testAPIKey, unreachable, unreachable, unreachable)

	_, uvErr := c.FetchUVForecast(context.Background(), 60.17, 24.94, time.Now())
	_, wmsErr := c.FetchWMSTile(context.Background(), wmsTileRequest())
	for name, err := range map[string]error{"UV": uvErr, "WMS": wmsErr} {
		if err == nil {
			t.Fatalf("%s: expected a transport error", name)
		}
		if strings.Contains(err.Error(), testAPIKey) {
			t.Errorf("%s error carries the API key: %v", name, err)
		}
		if !strings.Contains(err.Error(), "REDACTED") {
			t.Errorf("%s error should show where the key was removed: %v", name, err)
		}
	}
}

func TestStatusErrorIsTypedAndRedacted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden: "+r.URL.Path, http.StatusForbidden)
	}))
	defer srv.Close()
	c := NewClientWithWMS(srv.URL, testAPIKey, srv.URL, srv.URL, srv.URL)

	_, err := c.FetchUVForecast(context.Background(), 60.17, 24.94, time.Now())
	var status *StatusError
	if !errors.As(err, &status) {
		t.Fatalf("err = %v, want a *StatusError", err)
	}
	if status.Code != http.StatusForbidden || status.Service != "UV API" {
		t.Errorf("status = %d from %q, want 403 from UV API", status.Code, status.Service)
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Errorf("status error carries the API key echoed by the server: %v", err)
	}
	if IsRetryable(err) {
		t.Error("a 403 should not be retried")
	}
}

func TestFetchObservationsRetriesServerErrors(t *testing.T) {
	fastRetries(t)
	cases := []struct {
		name         string
		failures     int32
		status       int
		wantRequests int32
		wantErr      bool
	}{
		{"recovers after two 503s", 2, http.StatusServiceUnavailable, 3, false},
		{"gives up after the backoff schedule", 100, http.StatusServiceUnavailable, 4, true},
		{"throttling is retried", 1, http.StatusTooManyRequests, 2, false},
		{"a client error is not retried", 100, http.StatusBadRequest, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) <= tc.failures {
					http.Error(w, "nope", tc.status)
					return
				}
				w.Write([]byte(`<wfs:FeatureCollection xmlns:wfs="http://www.opengis.net/wfs/2.0"></wfs:FeatureCollection>`))
			}))
			defer srv.Close()

			_, err := NewClient(srv.URL, "", "").FetchObservations(context.Background())
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got := requests.Load(); got != tc.wantRequests {
				t.Errorf("made %d requests, want %d", got, tc.wantRequests)
			}
		})
	}
}

func TestRetryStopsWhenContextEnds(t *testing.T) {
	saved := retryBackoff
	retryBackoff = []time.Duration{time.Hour}
	defer func() { retryBackoff = saved }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	calls := 0
	err := Retry(ctx, func() error {
		calls++
		return &StatusError{Service: "FMI", Code: http.StatusBadGateway}
	})
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
		t.Fatalf("Retry = %v after %d calls, want the context error after 1", err, calls)
	}
}

func TestGetBodyRejectsOversizedResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "", "")

	if _, err := c.getBody(context.Background(), "FMI", srv.URL, 99); err == nil {
		t.Error("expected an error for a body over the limit")
	}
	body, err := c.getBody(context.Background(), "FMI", srv.URL, 100)
	if err != nil || len(body) != 100 {
		t.Errorf("getBody = %d bytes, %v; want 100, nil", len(body), err)
	}
}

func TestFetchWMSTileAcceptsOnlyPNG(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><ServiceExceptionReport><ServiceException>` + testAPIKey + `</ServiceException></ServiceExceptionReport>`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()
	c := NewClientWithWMS(srv.URL, testAPIKey, srv.URL, srv.URL, srv.URL)

	_, err := c.FetchWMSTile(context.Background(), wmsTileRequest())
	if err == nil {
		t.Fatal("a 200 answer that is not a PNG was accepted as a tile")
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Errorf("error carries the API key: %v", err)
	}

	body = append([]byte("\x89PNG\r\n\x1a\n"), "pixels"...)
	tile, err := c.FetchWMSTile(context.Background(), wmsTileRequest())
	if err != nil || len(tile) != len(body) {
		t.Fatalf("FetchWMSTile = %d bytes, %v; want the PNG", len(tile), err)
	}
}

func TestFetchUVForecastReportsUnparseableResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>maintenance</html>`))
	}))
	defer srv.Close()

	points, err := NewClient(srv.URL, testAPIKey, srv.URL).FetchUVForecast(context.Background(), 60.17, 24.94, time.Now())
	if err == nil {
		t.Fatalf("got %d points and no error for a response that is not JSON", len(points))
	}
}
