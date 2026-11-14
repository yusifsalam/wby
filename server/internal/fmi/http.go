package fmi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Upper bounds on FMI response bodies held in memory.
const (
	maxWFSBody     = 128 << 20
	maxUVBody      = 1 << 20
	maxTileBody    = 32 << 20
	errorBodyLimit = 2048
)

// StatusError is a non-200 answer from an FMI service.
type StatusError struct {
	Service string
	Code    int
	Body    string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s returned %d: %s", e.Service, e.Code, e.Body)
}

// get issues a GET and returns the response when the service answered 200,
// and a *StatusError otherwise. The API key is removed from any error.
func (c *Client) get(ctx context.Context, service, reqURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, errors.New(c.redact(err.Error()))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			urlErr.URL = c.redact(urlErr.URL)
		}
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit))
		resp.Body.Close()
		return nil, &StatusError{Service: service, Code: resp.StatusCode, Body: c.redact(string(body))}
	}
	return resp, nil
}

// getBody is get with the body read into memory. A body larger than limit is
// an error.
func (c *Client) getBody(ctx context.Context, service, reqURL string, limit int64) ([]byte, error) {
	resp, err := c.get(ctx, service, reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", service, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s response exceeds %d bytes", service, limit)
	}
	return data, nil
}

func (c *Client) redact(s string) string {
	if c.apiKey == "" {
		return s
	}
	return strings.ReplaceAll(s, c.apiKey, "REDACTED")
}

var retryBackoff = []time.Duration{2 * time.Second, 8 * time.Second, 30 * time.Second}

// Retry calls fn until it succeeds, fails with an error IsRetryable rejects,
// or has been retried after each wait in the backoff schedule.
func Retry(ctx context.Context, fn func() error) error {
	for attempt := 0; ; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		if attempt >= len(retryBackoff) || !IsRetryable(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryBackoff[attempt]):
		}
	}
}

// IsRetryable reports whether a failed FMI call is worth repeating: throttling
// (429), server-side failures (5xx) and network errors are; other statuses
// and cancellation are not.
func IsRetryable(err error) bool {
	var status *StatusError
	if errors.As(err, &status) {
		return status.Code == http.StatusTooManyRequests || status.Code >= 500
	}
	return !errors.Is(err, context.Canceled)
}
