package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"wby/internal/weather"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	saved := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(saved) })
	return &out
}

func logLines(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if raw == "" {
			continue
		}
		var line map[string]any
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatalf("log line is not JSON: %s", raw)
		}
		lines = append(lines, line)
	}
	return lines
}

func TestAccessLogRecordsRejectedRequestsWithoutTheirQuery(t *testing.T) {
	out := captureLogs(t)
	signed := NewRequestSignatureMiddleware(map[string]string{"web": "secret"}, time.Minute)
	h := Chain(http.NotFoundHandler(), AccessLog, signed)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/weather?lat=60.1699&lon=24.9384", nil)
	req.Header.Set("X-Client-ID", "web")
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for an unsigned request", rr.Code)
	}
	id := rr.Header().Get("X-Request-ID")
	if len(id) != 16 {
		t.Errorf("X-Request-ID = %q, want a generated 16-character id", id)
	}
	lines := logLines(t, out)
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %s", len(lines), out.String())
	}
	line := lines[0]
	if line["status"] != float64(http.StatusUnauthorized) || line["path"] != "/v1/weather" || line["client"] != "web" || line["request_id"] != id {
		t.Errorf("unexpected access log line: %v", line)
	}
	if strings.Contains(out.String(), "60.1699") || strings.Contains(out.String(), "24.9384") {
		t.Errorf("access log carries the request coordinates: %s", out.String())
	}
}

func TestAccessLogKeepsACallerSuppliedRequestID(t *testing.T) {
	captureLogs(t)
	h := Chain(http.NotFoundHandler(), AccessLog)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/weather", nil)
	req.Header.Set("X-Request-ID", "edge-1234.abc")
	h.ServeHTTP(rr, req)
	if got := rr.Header().Get("X-Request-ID"); got != "edge-1234.abc" {
		t.Errorf("X-Request-ID = %q, want the caller's", got)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/weather", nil)
	req.Header.Set("X-Request-ID", "bad id\"with junk")
	h.ServeHTTP(rr, req)
	if got := rr.Header().Get("X-Request-ID"); got == "" || strings.ContainsAny(got, " \"") {
		t.Errorf("X-Request-ID = %q, want a generated id in place of an unsafe one", got)
	}
}

func TestAccessLogSkipsSuccessfulHealthProbes(t *testing.T) {
	out := captureLogs(t)
	mux := http.NewServeMux()
	handler := NewHandler(weatherServiceStub{})
	handler.SetHealthChecks(HealthCheck{Name: "database", Critical: true, Probe: func(context.Context) HealthResult {
		return HealthResult{Detail: "ping failed"}
	}})
	handler.RegisterRoutes(mux)
	h := Chain(mux, AccessLog)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	if lines := logLines(t, out); len(lines) != 0 {
		t.Fatalf("a successful health probe was logged: %v", lines)
	}

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	lines := logLines(t, out)
	if len(lines) != 1 || lines[0]["status"] != float64(http.StatusServiceUnavailable) {
		t.Fatalf("a failing readiness probe should be logged once with 503, got %v", lines)
	}
}

func TestRecoverAnswers500ForAPanickingHandler(t *testing.T) {
	out := captureLogs(t)
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), AccessLog, Recover)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/weather", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body["error"] == "" {
		t.Errorf("body = %q, want a JSON error", rr.Body.String())
	}
	var panicLogged, accessLogged bool
	for _, line := range logLines(t, out) {
		if line["msg"] == "handler panicked" && line["panic"] == "boom" && line["stack"] != "" {
			panicLogged = true
		}
		if line["msg"] == "request" && line["status"] == float64(http.StatusInternalServerError) {
			accessLogged = true
		}
	}
	if !panicLogged || !accessLogged {
		t.Errorf("want the panic and the 500 logged, got: %s", out.String())
	}
}

type slowWeatherService struct {
	weatherServiceStub
}

func (slowWeatherService) GetWeather(ctx context.Context, lat, lon float64) (*weather.WeatherResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRequestTimeoutAnswers504(t *testing.T) {
	captureLogs(t)
	mux := http.NewServeMux()
	NewHandler(slowWeatherService{}).RegisterRoutes(mux)
	h := Chain(mux, Timeout(30*time.Millisecond))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/weather?lat=60.1&lon=24.9", nil))

	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504 when the request runs out of time", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "error") {
		t.Errorf("body = %q, want a JSON error", rr.Body.String())
	}
}

func TestClientDisconnectWritesNothing(t *testing.T) {
	captureLogs(t)
	mux := http.NewServeMux()
	NewHandler(slowWeatherService{}).RegisterRoutes(mux)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/weather?lat=60.1&lon=24.9", nil).WithContext(ctx))

	if rr.Body.Len() != 0 {
		t.Errorf("wrote %q to a client that already left", rr.Body.String())
	}
}

func TestReadiness(t *testing.T) {
	refreshed := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	ok := func(context.Context) HealthResult { return HealthResult{OK: true} }
	failing := func(context.Context) HealthResult { return HealthResult{Detail: "ping failed"} }
	fresh := func(context.Context) HealthResult {
		return FreshnessResult(refreshed, 30*time.Minute, refreshed.Add(10*time.Minute))
	}
	stale := func(context.Context) HealthResult {
		return FreshnessResult(refreshed, 30*time.Minute, refreshed.Add(45*time.Minute))
	}
	never := func(context.Context) HealthResult { return FreshnessResult(time.Time{}, 30*time.Minute, refreshed) }

	cases := []struct {
		name       string
		checks     []HealthCheck
		wantStatus int
	}{
		{"no checks configured", nil, http.StatusOK},
		{"everything healthy", []HealthCheck{{Name: "database", Critical: true, Probe: ok}, {Name: "observations", Critical: true, Probe: fresh}}, http.StatusOK},
		{"database down", []HealthCheck{{Name: "database", Critical: true, Probe: failing}}, http.StatusServiceUnavailable},
		{"observations stale", []HealthCheck{{Name: "observations", Critical: true, Probe: stale}}, http.StatusServiceUnavailable},
		{"no observation fetch yet", []HealthCheck{{Name: "observations", Critical: true, Probe: never}}, http.StatusServiceUnavailable},
		{"only a non-critical check failing", []HealthCheck{{Name: "database", Critical: true, Probe: ok}, {Name: "radar", Probe: stale}}, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(weatherServiceStub{})
			h.SetHealthChecks(tc.checks...)
			rr := httptest.NewRecorder()
			h.ready(rr, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rr.Code, tc.wantStatus, rr.Body.String())
			}
			var resp struct {
				Status string                  `json:"status"`
				Checks map[string]HealthResult `json:"checks"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if (resp.Status == "ok") != (tc.wantStatus == http.StatusOK) || len(resp.Checks) != len(tc.checks) {
				t.Errorf("body = %s", rr.Body.String())
			}
		})
	}

	h := NewHandler(weatherServiceStub{})
	h.SetHealthChecks(HealthCheck{Name: "observations", Critical: true, Probe: stale})
	rr := httptest.NewRecorder()
	h.ready(rr, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if !strings.Contains(rr.Body.String(), `"at":"2026-11-14T12:00:00Z"`) || !strings.Contains(rr.Body.String(), "older than 30m0s") {
		t.Errorf("a stale check should report when its data was last refreshed: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.health(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("liveness = %d while not ready, want 200", rr.Code)
	}
}
