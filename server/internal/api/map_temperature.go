package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"wby/internal/weather"
)

const (
	minOverlayDim = 64
	maxOverlayDim = 1600
)

func (h *Handler) getTemperatureOverlay(w http.ResponseWriter, r *http.Request) {
	req, err := parseMapTemperatureRequest(r)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	overlay, err := h.service.GetTemperatureOverlay(r.Context(), req)
	if err != nil {
		slog.Error("get temperature overlay failed", "err", err, "bbox", fmt.Sprintf("%f,%f,%f,%f", req.MinLon, req.MinLat, req.MaxLon, req.MaxLat))
		writeJSONError(w, "overlay unavailable", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=120")
	w.Header().Set("X-Data-Time", overlay.DataTime.UTC().Format(time.RFC3339))
	w.Header().Set("X-Temp-Min", strconv.FormatFloat(overlay.MinTemp, 'f', 2, 64))
	w.Header().Set("X-Temp-Max", strconv.FormatFloat(overlay.MaxTemp, 'f', 2, 64))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(overlay.PNG)
}

func parseMapTemperatureRequest(r *http.Request) (weather.MapOverlayRequest, error) {
	bboxRaw := strings.TrimSpace(r.URL.Query().Get("bbox"))
	parts := strings.Split(bboxRaw, ",")
	if len(parts) != 4 {
		return weather.MapOverlayRequest{}, fmt.Errorf("invalid bbox parameter")
	}

	var bbox [4]float64
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return weather.MapOverlayRequest{}, fmt.Errorf("invalid bbox parameter")
		}
		bbox[i] = v
	}

	width, err := strconv.Atoi(r.URL.Query().Get("width"))
	if err != nil {
		return weather.MapOverlayRequest{}, fmt.Errorf("invalid width parameter")
	}
	height, err := strconv.Atoi(r.URL.Query().Get("height"))
	if err != nil {
		return weather.MapOverlayRequest{}, fmt.Errorf("invalid height parameter")
	}
	width = clamp(width, minOverlayDim, maxOverlayDim)
	height = clamp(height, minOverlayDim, maxOverlayDim)

	req := weather.MapOverlayRequest{
		MinLon: bbox[0],
		MinLat: bbox[1],
		MaxLon: bbox[2],
		MaxLat: bbox[3],
		Width:  width,
		Height: height,
	}
	if err := validateBBox(&req); err != nil {
		return weather.MapOverlayRequest{}, err
	}
	return req, nil
}

func validateBBox(req *weather.MapOverlayRequest) error {
	req.MinLon = clampFloat(req.MinLon, -180, 180)
	req.MaxLon = clampFloat(req.MaxLon, -180, 180)
	req.MinLat = clampFloat(req.MinLat, -90, 90)
	req.MaxLat = clampFloat(req.MaxLat, -90, 90)

	if req.MinLon >= req.MaxLon || req.MinLat >= req.MaxLat {
		return fmt.Errorf("invalid bbox parameter")
	}
	return nil
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
