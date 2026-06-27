package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"wby/internal/weather"
)

type temperatureSamplesJSON struct {
	DataTime time.Time               `json:"data_time"`
	MinTemp  float64                 `json:"min_temp"`
	MaxTemp  float64                 `json:"max_temp"`
	Samples  []temperatureSampleJSON `json:"samples"`
	Grid     *temperatureGridJSON    `json:"grid,omitempty"`
}

type temperatureSampleJSON struct {
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Temp       float64   `json:"temp"`
	ObservedAt time.Time `json:"observed_at"`
}

// temperatureGridJSON is the dense GRIB raster the client uploads as a texture:
// a regular lat/lon grid, row-major north-to-south (row 0 = max_lat),
// west-to-east, length rows*cols, in Celsius; null = masked.
type temperatureGridJSON struct {
	Rows   int        `json:"rows"`
	Cols   int        `json:"cols"`
	MinLat float64    `json:"min_lat"`
	MaxLat float64    `json:"max_lat"`
	MinLon float64    `json:"min_lon"`
	MaxLon float64    `json:"max_lon"`
	Values []*float64 `json:"values"`
}

func (h *Handler) getTemperatureSamples(w http.ResponseWriter, r *http.Request) {
	var (
		at         time.Time
		atProvided bool
	)
	if raw := r.URL.Query().Get("at"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeJSONError(w, "invalid at parameter (expected RFC3339)", http.StatusBadRequest)
			return
		}
		// Don't snap here: rounding 12:31 to 13:00 can flip a historical
		// request into the future-forecast path. The forecast store
		// truncates to hour internally; observation queries use a ±30min
		// window around the requested instant.
		at = parsed.UTC()
		atProvided = true

		// Cap future requests at the backfill horizon. Beyond it, no amount
		// of fan-out can produce data, so reject up-front instead of spinning
		// useless backfills on every retry.
		horizon := time.Duration(weather.ForecastBackfillHorizon) * time.Hour
		if at.After(time.Now().Add(horizon)) {
			writeJSONError(w, "at exceeds forecast horizon", http.StatusBadRequest)
			return
		}
	}

	var (
		resp *weather.TemperatureSamplesResponse
		err  error
	)
	if atProvided {
		resp, err = h.service.GetTemperatureSamplesAt(r.Context(), at)
	} else {
		resp, err = h.service.GetTemperatureSamples(r.Context())
	}
	if err != nil {
		if isClientCanceled(err) {
			return
		}
		slog.Error("get temperature samples failed", "err", err, "at", at)
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
	switch {
	case atProvided && at.After(time.Now()) && resp.Grid == nil && len(resp.Samples) < weather.ForecastBackfillThreshold:
		// Sparse future response (station fallback) just scheduled a backfill.
		// Keep the cache window tight so the next request picks up the denser
		// refill instead of clients/proxies pinning the sparse payload. The dense
		// GRIB grid is never sparse, so it takes the normal future cache below.
		w.Header().Set("Cache-Control", "public, max-age=15, stale-while-revalidate=60")
	case atProvided:
		w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=900")
	default:
		w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	}
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
		Grid:     buildTemperatureGridJSON(resp.Grid),
	}
}

func buildTemperatureGridJSON(grid *weather.FieldGrid) *temperatureGridJSON {
	if grid == nil {
		return nil
	}
	values := make([]*float64, len(grid.Values))
	for i, v := range grid.Values {
		if v == nil {
			continue
		}
		// Round to 0.1°C — plenty for an overlay and roughly halves the payload.
		rounded := math.Round(*v*10) / 10
		values[i] = &rounded
	}
	return &temperatureGridJSON{
		Rows:   grid.Rows,
		Cols:   grid.Cols,
		MinLat: grid.MinLat,
		MaxLat: grid.MaxLat,
		MinLon: grid.MinLon,
		MaxLon: grid.MaxLon,
		Values: values,
	}
}
