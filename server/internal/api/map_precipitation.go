package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"wby/internal/weather"
)

func (h *Handler) getPrecipitationOverlay(w http.ResponseWriter, r *http.Request) {
	base, err := parseMapTemperatureRequest(r)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	const precipStep = 5 * time.Minute
	target := time.Now().UTC().Truncate(precipStep)
	if raw := strings.TrimSpace(r.URL.Query().Get("time")); raw != "" {
		t, perr := time.Parse(time.RFC3339, raw)
		if perr != nil {
			writeJSONError(w, "invalid time parameter", http.StatusBadRequest)
			return
		}
		target = t.UTC().Truncate(precipStep)
	}

	overlay, err := h.service.GetPrecipitationOverlay(r.Context(), weather.PrecipitationOverlayRequest{
		MapOverlayRequest: base,
		Time:              target,
	})
	if err != nil {
		if errors.Is(err, weather.ErrPrecipitationDisabled) {
			writeJSONError(w, "precipitation overlay not configured", http.StatusNotFound)
			return
		}
		slog.Error("get precipitation overlay failed", "err", err, "time", target)
		writeJSONError(w, "overlay unavailable", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=120")
	w.Header().Set("X-Data-Time", overlay.DataTime.UTC().Format(time.RFC3339))
	w.Header().Set("X-Layer", overlay.Layer)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(overlay.PNG)
}
