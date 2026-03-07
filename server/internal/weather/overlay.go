package weather

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"time"
)

const (
	overlayMinSamples = 3
	idwEpsilon        = 1e-6
)

func RenderTemperatureOverlay(req MapOverlayRequest, samples []TemperatureSample) (*TemperatureOverlay, error) {
	if req.Width <= 0 || req.Height <= 0 {
		return nil, fmt.Errorf("invalid output size")
	}
	if req.MinLon >= req.MaxLon || req.MinLat >= req.MaxLat {
		return nil, fmt.Errorf("invalid bbox")
	}
	if len(samples) < overlayMinSamples {
		return nil, fmt.Errorf("not enough samples")
	}

	minTemp := samples[0].Temperature
	maxTemp := samples[0].Temperature
	dataTime := samples[0].ObservedAt
	for _, s := range samples[1:] {
		if s.Temperature < minTemp {
			minTemp = s.Temperature
		}
		if s.Temperature > maxTemp {
			maxTemp = s.Temperature
		}
		if s.ObservedAt.After(dataTime) {
			dataTime = s.ObservedAt
		}
	}
	if maxTemp == minTemp {
		maxTemp = minTemp + 0.1
	}

	img := image.NewNRGBA(image.Rect(0, 0, req.Width, req.Height))
	latSpan := req.MaxLat - req.MinLat
	lonSpan := req.MaxLon - req.MinLon

	for y := 0; y < req.Height; y++ {
		v := float64(y) / float64(maxInt(req.Height-1, 1))
		lat := req.MaxLat - v*latSpan
		cosLat := math.Cos(lat * math.Pi / 180.0)
		for x := 0; x < req.Width; x++ {
			u := float64(x) / float64(maxInt(req.Width-1, 1))
			lon := req.MinLon + u*lonSpan

			temp, nearest := interpolateTemperature(samples, lat, lon, cosLat)
			if math.IsNaN(temp) || math.IsInf(temp, 0) {
				continue
			}

			tNorm := (temp - minTemp) / (maxTemp - minTemp)
			base := rampColor(tNorm)
			alpha := alphaForNearest(nearest)
			img.SetNRGBA(x, y, color.NRGBA{R: base.R, G: base.G, B: base.B, A: alpha})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}

	return &TemperatureOverlay{
		PNG:      buf.Bytes(),
		DataTime: dataTime.UTC().Truncate(time.Second),
		MinTemp:  minTemp,
		MaxTemp:  maxTemp,
	}, nil
}

func interpolateTemperature(samples []TemperatureSample, lat, lon, cosLat float64) (float64, float64) {
	var (
		sumW      float64
		sumTemp   float64
		nearestSq = math.MaxFloat64
	)
	for _, s := range samples {
		dLat := lat - s.Lat
		dLon := (lon - s.Lon) * cosLat
		distSq := dLat*dLat + dLon*dLon
		if distSq < nearestSq {
			nearestSq = distSq
		}
		w := 1.0 / (distSq + idwEpsilon)
		sumW += w
		sumTemp += w * s.Temperature
	}
	if sumW == 0 {
		return math.NaN(), nearestSq
	}
	return sumTemp / sumW, nearestSq
}

func rampColor(v float64) color.NRGBA {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	stops := []color.NRGBA{
		{R: 34, G: 74, B: 204, A: 255},
		{R: 38, G: 166, B: 229, A: 255},
		{R: 73, G: 205, B: 139, A: 255},
		{R: 245, G: 196, B: 67, A: 255},
		{R: 236, G: 98, B: 70, A: 255},
	}
	step := 1.0 / float64(len(stops)-1)
	idx := int(v / step)
	if idx >= len(stops)-1 {
		return stops[len(stops)-1]
	}
	localT := (v - float64(idx)*step) / step
	a := stops[idx]
	b := stops[idx+1]
	return color.NRGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*localT),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*localT),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*localT),
		A: 255,
	}
}

func alphaForNearest(nearestDistSq float64) uint8 {
	// Distances are in degrees^2; this keeps edges from looking hard-cut near sparse stations.
	switch {
	case nearestDistSq <= 0.001:
		return 210
	case nearestDistSq <= 0.003:
		return 190
	case nearestDistSq <= 0.008:
		return 170
	case nearestDistSq <= 0.02:
		return 140
	default:
		return 110
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
