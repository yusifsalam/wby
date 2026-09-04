package weather

import "math"

// FeelsLike returns the apparent temperature from air temperature (°C) and
// wind speed (m/s) using the wind chill formula. It falls back to the air
// temperature outside wind chill conditions or when wind is unknown.
func FeelsLike(temp, wind *float64) *float64 {
	if temp == nil || wind == nil {
		return temp
	}
	t := *temp
	w := *wind * 3.6
	if t > 10 || w < 4.8 {
		return temp
	}
	fl := 13.12 + 0.6215*t - 11.37*math.Pow(w, 0.16) + 0.3965*t*math.Pow(w, 0.16)
	return &fl
}
