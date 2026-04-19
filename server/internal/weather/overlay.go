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
	colorScaleMinTemp = -40.0
	colorScaleMaxTemp = 40.0
	overlayBaseAlpha  = 195
	coverageInnerDist = 0.35
	coverageOuterDist = 1.10
	mercatorMaxLat    = 85.05112878
)

var temperatureColorStops = []struct {
	temp float64
	rgb  color.NRGBA
}{
	{temp: -40, rgb: color.NRGBA{R: 80, G: 30, B: 130, A: 255}},
	{temp: -20, rgb: color.NRGBA{R: 30, G: 55, B: 150, A: 255}},
	{temp: -10, rgb: color.NRGBA{R: 55, G: 115, B: 220, A: 255}},
	{temp: 0, rgb: color.NRGBA{R: 80, G: 190, B: 180, A: 255}},
	{temp: 10, rgb: color.NRGBA{R: 180, G: 215, B: 75, A: 255}},
	{temp: 20, rgb: color.NRGBA{R: 245, G: 210, B: 55, A: 255}},
	{temp: 30, rgb: color.NRGBA{R: 210, G: 50, B: 40, A: 255}},
	{temp: 40, rgb: color.NRGBA{R: 109, G: 22, B: 11, A: 255}},
}

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
	img := image.NewNRGBA(image.Rect(0, 0, req.Width, req.Height))
	lonSpan := req.MaxLon - req.MinLon

	for y := 0; y < req.Height; y++ {
		lat := latitudeAtRow(req.MinLat, req.MaxLat, y, req.Height)
		cosLat := math.Cos(lat * math.Pi / 180.0)
		for x := 0; x < req.Width; x++ {
			u := float64(x) / float64(maxInt(req.Width-1, 1))
			lon := req.MinLon + u*lonSpan

			temp, nearest := interpolateTemperature(samples, lat, lon, cosLat)
			if math.IsNaN(temp) || math.IsInf(temp, 0) {
				continue
			}

			base := rampColorForTemperature(temp)
			alpha := alphaForCoverage(nearest)
			if alpha == 0 {
				continue
			}
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

func normalizeTemperature(temp float64) float64 {
	v := (temp - colorScaleMinTemp) / (colorScaleMaxTemp - colorScaleMinTemp)
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func alphaForCoverage(nearestDistSq float64) uint8 {
	if nearestDistSq <= 0 {
		return overlayBaseAlpha
	}

	dist := math.Sqrt(nearestDistSq)
	if dist >= coverageOuterDist {
		return 0
	}
	if dist <= coverageInnerDist {
		return overlayBaseAlpha
	}

	t := 1 - (dist-coverageInnerDist)/(coverageOuterDist-coverageInnerDist)
	if t <= 0 {
		return 0
	}

	// Smoothstep to avoid visible rings at coverage edges.
	smooth := t * t * (3 - 2*t)
	a := smooth * float64(overlayBaseAlpha)
	if a < 1 {
		return 0
	}
	if a > 255 {
		return 255
	}
	return uint8(a + 0.5)
}

func latitudeAtRow(minLat, maxLat float64, y, height int) float64 {
	if height <= 1 {
		return clampMercatorLat((minLat + maxLat) / 2)
	}

	v := float64(y) / float64(height-1)
	maxY := mercatorY(maxLat)
	minY := mercatorY(minLat)
	yMerc := maxY - v*(maxY-minY)
	return inverseMercatorLat(yMerc)
}

func mercatorY(lat float64) float64 {
	lat = clampMercatorLat(lat)
	rad := lat * math.Pi / 180.0
	return math.Log(math.Tan(math.Pi/4 + rad/2))
}

func inverseMercatorLat(y float64) float64 {
	rad := 2*math.Atan(math.Exp(y)) - math.Pi/2
	lat := rad * 180.0 / math.Pi
	return clampMercatorLat(lat)
}

func clampMercatorLat(lat float64) float64 {
	if lat > mercatorMaxLat {
		return mercatorMaxLat
	}
	if lat < -mercatorMaxLat {
		return -mercatorMaxLat
	}
	return lat
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

func rampColorForTemperature(temp float64) color.NRGBA {
	if temp <= temperatureColorStops[0].temp {
		return temperatureColorStops[0].rgb
	}
	lastIdx := len(temperatureColorStops) - 1
	if temp >= temperatureColorStops[lastIdx].temp {
		return temperatureColorStops[lastIdx].rgb
	}

	for i := 0; i < lastIdx; i++ {
		a := temperatureColorStops[i]
		b := temperatureColorStops[i+1]
		if temp > b.temp {
			continue
		}

		t := (temp - a.temp) / (b.temp - a.temp)
		return color.NRGBA{
			R: uint8(float64(a.rgb.R) + (float64(b.rgb.R)-float64(a.rgb.R))*t),
			G: uint8(float64(a.rgb.G) + (float64(b.rgb.G)-float64(a.rgb.G))*t),
			B: uint8(float64(a.rgb.B) + (float64(b.rgb.B)-float64(a.rgb.B))*t),
			A: 255,
		}
	}

	return temperatureColorStops[lastIdx].rgb
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
