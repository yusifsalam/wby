package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type leaderboardJSON struct {
	Timeframe   string               `json:"timeframe"`
	Leaderboard []leaderboardEntryJSON `json:"leaderboard"`
}

type leaderboardEntryJSON struct {
	Type        string    `json:"type"`
	StationName string    `json:"station_name"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	DistanceKM  float64   `json:"distance_km"`
	ObservedAt  time.Time `json:"observed_at"`
}

var supportedTimeframes = map[string]bool{
	"now": true,
	"1h":  true,
	"24h": true,
	"3d":  true,
	"7d":  true,
}

func (h *Handler) getLeaderboard(w http.ResponseWriter, r *http.Request) {
	lat, err := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	if err != nil {
		writeJSONError(w, "invalid lat parameter", http.StatusBadRequest)
		return
	}
	lon, err := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	if err != nil {
		writeJSONError(w, "invalid lon parameter", http.StatusBadRequest)
		return
	}

	timeframe := r.URL.Query().Get("timeframe")
	if timeframe == "" {
		timeframe = "now"
	}
	if !supportedTimeframes[timeframe] {
		writeJSONError(w, "unsupported timeframe: "+timeframe, http.StatusBadRequest)
		return
	}

	entries, err := h.service.GetLeaderboard(r.Context(), lat, lon, timeframe)
	if err != nil {
		slog.Error("get leaderboard failed", "err", err, "lat", lat, "lon", lon)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := leaderboardJSON{
		Timeframe:   timeframe,
		Leaderboard: make([]leaderboardEntryJSON, len(entries)),
	}
	for i, e := range entries {
		resp.Leaderboard[i] = leaderboardEntryJSON{
			Type:        e.StatType,
			StationName: e.StationName,
			Lat:         e.Lat,
			Lon:         e.Lon,
			Value:       e.Value,
			Unit:        e.Unit,
			DistanceKM:  e.DistanceKM,
			ObservedAt:  e.ObservedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=120")
	json.NewEncoder(w).Encode(resp)
}
