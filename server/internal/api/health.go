package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthCheck is one condition reported by /health/ready.
type HealthCheck struct {
	Name string
	// Critical checks make the service not ready when they fail; the others
	// are reported only.
	Critical bool
	Probe    func(ctx context.Context) HealthResult
}

// HealthResult is the outcome of one HealthCheck.
type HealthResult struct {
	OK bool `json:"ok"`
	// At is when the checked data was last refreshed, when that applies.
	At     *time.Time `json:"at,omitempty"`
	Detail string     `json:"detail,omitempty"`
}

const healthProbeTimeout = 3 * time.Second

// SetHealthChecks configures what /health/ready reports. Without any, it
// answers ready.
func (h *Handler) SetHealthChecks(checks ...HealthCheck) {
	h.healthChecks = checks
}

type readinessJSON struct {
	Status string                  `json:"status"`
	Checks map[string]HealthResult `json:"checks"`
}

// health reports that the process is serving requests.
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// ready reports whether the service has what it needs to answer with current
// data: 200 when every critical check passes, 503 otherwise.
func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthProbeTimeout)
	defer cancel()

	resp := readinessJSON{Status: "ok", Checks: make(map[string]HealthResult, len(h.healthChecks))}
	status := http.StatusOK
	for _, check := range h.healthChecks {
		result := check.Probe(ctx)
		resp.Checks[check.Name] = result
		if !result.OK && check.Critical {
			resp.Status = "unavailable"
			status = http.StatusServiceUnavailable
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// FreshnessResult is a HealthResult for data last refreshed at the given
// time: OK when that is within maxAge, with the time reported.
func FreshnessResult(at time.Time, maxAge time.Duration, now time.Time) HealthResult {
	if at.IsZero() {
		return HealthResult{Detail: "not refreshed since startup"}
	}
	at = at.UTC().Truncate(time.Second)
	if now.Sub(at) > maxAge {
		return HealthResult{At: &at, Detail: "older than " + maxAge.String()}
	}
	return HealthResult{OK: true, At: &at}
}
