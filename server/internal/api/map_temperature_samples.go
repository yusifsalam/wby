package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"wby/internal/weather"
)

type temperatureSamplesJSON struct {
	DataTime time.Time               `json:"data_time"`
	MinTemp  float64                 `json:"min_temp"`
	MaxTemp  float64                 `json:"max_temp"`
	Samples  []temperatureSampleJSON `json:"samples"`
}

type temperatureSampleJSON struct {
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Temp       float64   `json:"temp"`
	ObservedAt time.Time `json:"observed_at"`
}

func (h *Handler) getTemperatureSamples(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.GetTemperatureSamples(r.Context())
	if err != nil {
		slog.Error("get temperature samples failed", "err", err)
		writeJSONError(w, "samples unavailable", http.StatusBadGateway)
		return
	}

	payload := buildTemperatureSamplesJSON(resp)
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("marshal temperature samples failed", "err", err)
		writeJSONError(w, "samples unavailable", http.StatusBadGateway)
		return
	}

	digest := sha256.Sum256(body)
	etag := fmt.Sprintf(`"%x"`, digest)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func buildTemperatureSamplesJSON(resp *weather.TemperatureSamplesResponse) temperatureSamplesJSON {
	samples := make([]temperatureSampleJSON, len(resp.Samples))
	for i, sample := range resp.Samples {
		samples[i] = temperatureSampleJSON{
			Lat:        sample.Lat,
			Lon:        sample.Lon,
			Temp:       sample.Temperature,
			ObservedAt: sample.ObservedAt.UTC().Truncate(time.Second),
		}
	}
	return temperatureSamplesJSON{
		DataTime: resp.DataTime,
		MinTemp:  resp.MinTemp,
		MaxTemp:  resp.MaxTemp,
		Samples:  samples,
	}
}
