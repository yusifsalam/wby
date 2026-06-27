package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"wby/internal/weather"
)

func (h *Handler) getPrecipitationOverlay(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	base, err := parseMapTemperatureRequest(r)
	if err != nil {
		slog.Info("precipitation overlay request completed",
			"status", http.StatusBadRequest,
			"duration_ms", time.Since(started).Milliseconds(),
			"err", err.Error(),
		)
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	const precipStep = 5 * time.Minute
	target := time.Now().UTC().Truncate(precipStep)
	if raw := strings.TrimSpace(r.URL.Query().Get("time")); raw != "" {
		t, perr := time.Parse(time.RFC3339, raw)
		if perr != nil {
			slog.Info("precipitation overlay request completed",
				"status", http.StatusBadRequest,
				"duration_ms", time.Since(started).Milliseconds(),
				"bbox", formatOverlayBBox(base),
				"width", base.Width,
				"height", base.Height,
				"err", "invalid time parameter",
			)
			writeJSONError(w, "invalid time parameter", http.StatusBadRequest)
			return
		}
		target = t.UTC().Truncate(precipStep)
	}

	status := http.StatusOK
	layer := ""
	errText := ""
	defer func() {
		attrs := []any{
			"status", status,
			"duration_ms", time.Since(started).Milliseconds(),
			"target", target.UTC().Format(time.RFC3339),
			"bbox", formatOverlayBBox(base),
			"width", base.Width,
			"height", base.Height,
		}
		if layer != "" {
			attrs = append(attrs, "layer", layer)
		}
		if errText != "" {
			attrs = append(attrs, "err", errText)
		}
		slog.Info("precipitation overlay request completed", attrs...)
	}()

	overlay, err := h.service.GetPrecipitationOverlay(r.Context(), weather.PrecipitationOverlayRequest{
		MapOverlayRequest: base,
		Time:              target,
	})
	if err != nil {
		if errors.Is(err, weather.ErrPrecipitationDisabled) {
			status = http.StatusNotFound
			errText = "precipitation overlay not configured"
			writeJSONError(w, "precipitation overlay not configured", http.StatusNotFound)
			return
		}
		if isClientCanceled(err) {
			status = 499
			errText = err.Error()
			return
		}
		status = http.StatusBadGateway
		errText = err.Error()
		slog.Error("get precipitation overlay failed", "err", err, "time", target)
		writeJSONError(w, "overlay unavailable", http.StatusBadGateway)
		return
	}
	layer = overlay.Layer

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=120")
	w.Header().Set("X-Data-Time", overlay.DataTime.UTC().Format(time.RFC3339))
	w.Header().Set("X-Layer", overlay.Layer)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(overlay.PNG)
}

func (h *Handler) getPrecipitationForecastOverlay(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	base, err := parseMapTemperatureRequest(r)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Hourly steps, matching the Harmonie field cadence. A zero target means
	// "current hour" (resolved in the service).
	const precipStep = time.Hour
	var target time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("time")); raw != "" {
		t, perr := time.Parse(time.RFC3339, raw)
		if perr != nil {
			writeJSONError(w, "invalid time parameter", http.StatusBadRequest)
			return
		}
		target = t.UTC().Truncate(precipStep)
	}

	status := http.StatusOK
	errText := ""
	defer func() {
		attrs := []any{
			"status", status,
			"duration_ms", time.Since(started).Milliseconds(),
			"target", target.UTC().Format(time.RFC3339),
			"bbox", formatOverlayBBox(base),
			"width", base.Width,
			"height", base.Height,
		}
		if errText != "" {
			attrs = append(attrs, "err", errText)
		}
		slog.Info("precipitation forecast overlay request completed", attrs...)
	}()

	overlay, err := h.service.GetPrecipitationForecastOverlay(r.Context(), weather.PrecipitationOverlayRequest{
		MapOverlayRequest: base,
		Time:              target,
	})
	if err != nil {
		if errors.Is(err, weather.ErrPrecipitationDisabled) {
			status = http.StatusNotFound
			errText = "precipitation forecast not available"
			writeJSONError(w, "precipitation forecast not available", http.StatusNotFound)
			return
		}
		if isClientCanceled(err) {
			status = 499
			errText = err.Error()
			return
		}
		status = http.StatusBadGateway
		errText = err.Error()
		slog.Error("get precipitation forecast overlay failed", "err", err, "time", target)
		writeJSONError(w, "overlay unavailable", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Data-Time", overlay.DataTime.UTC().Format(time.RFC3339))
	w.Header().Set("X-Layer", overlay.Layer)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(overlay.PNG)
}

func formatOverlayBBox(req weather.MapOverlayRequest) string {
	return fmt.Sprintf("%.4f,%.4f,%.4f,%.4f", req.MinLon, req.MinLat, req.MaxLon, req.MaxLat)
}
