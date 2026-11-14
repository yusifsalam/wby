package api

import (
	"math"
	"net/http"
	"strconv"
)

// parseLatLon reads the lat and lon query parameters. When either is missing,
// not a finite number, or outside its valid range, it writes a 400 response
// and returns ok false.
func parseLatLon(w http.ResponseWriter, r *http.Request) (lat, lon float64, ok bool) {
	lat, ok = parseCoordinate(r.URL.Query().Get("lat"), 90)
	if !ok {
		writeJSONError(w, "invalid lat parameter", http.StatusBadRequest)
		return 0, 0, false
	}
	lon, ok = parseCoordinate(r.URL.Query().Get("lon"), 180)
	if !ok {
		writeJSONError(w, "invalid lon parameter", http.StatusBadRequest)
		return 0, 0, false
	}
	return lat, lon, true
}

func parseCoordinate(raw string, limit float64) (float64, bool) {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(v) || math.Abs(v) > limit {
		return 0, false
	}
	return v, true
}
