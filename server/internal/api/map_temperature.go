package api

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"wby/internal/weather"
)

const (
	minOverlayDim = 64
	maxOverlayDim = 1600
)

func parseMapTemperatureRequest(r *http.Request) (weather.MapOverlayRequest, error) {
	bboxRaw := strings.TrimSpace(r.URL.Query().Get("bbox"))
	parts := strings.Split(bboxRaw, ",")
	if len(parts) != 4 {
		return weather.MapOverlayRequest{}, fmt.Errorf("invalid bbox parameter")
	}

	var bbox [4]float64
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || math.IsNaN(v) {
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
