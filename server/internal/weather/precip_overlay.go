package weather

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"math"
	"time"
)

// precipForecastLayer is the cache/log label for the GRIB-backed precipitation
// forecast overlay (distinct from the WMS observation/forecast layers).
const precipForecastLayer = "harmonie:precipitationRate"

// GetPrecipitationForecastOverlay renders the Harmonie precipitation-rate field
// (mm/h) over the request bbox at the requested hour into a PNG overlay. The
// time is snapped to the hour; a zero time means the current hour. Returns
// ErrPrecipitationDisabled when the GRIB source is unconfigured or the field
// isn't available yet (so the handler can answer 404 and the client falls back
// to keeping its previous frame).
func (s *Service) GetPrecipitationForecastOverlay(ctx context.Context, req PrecipitationOverlayRequest) (*PrecipitationOverlay, error) {
	if s.gribPrecip == nil {
		return nil, ErrPrecipitationDisabled
	}

	target := req.Time.UTC().Truncate(time.Hour)
	if target.IsZero() {
		target = time.Now().UTC().Truncate(time.Hour)
	}

	cacheKey := precipCacheKey(precipForecastLayer, target, req.MapOverlayRequest)
	if cached, ok := s.gribPrecipCache.Get(cacheKey); ok {
		return cached, nil
	}

	samples, validTime, err := s.gribPrecip.Samples(ctx, req.MinLon, req.MinLat, req.MaxLon, req.MaxLat, target)
	if err != nil {
		return nil, fmt.Errorf("fetch precipitation samples: %w", err)
	}
	if len(samples) < overlayMinSamples {
		// Soft miss: file/field not available yet, or no rain anywhere in range.
		return nil, ErrPrecipitationDisabled
	}
	if validTime.IsZero() {
		validTime = target
	}

	overlay, err := RenderPrecipitationOverlay(req.MapOverlayRequest, samples, validTime)
	if err != nil {
		return nil, fmt.Errorf("render precipitation overlay: %w", err)
	}
	s.gribPrecipCache.Set(cacheKey, overlay)
	slog.Info("precipitation forecast overlay rendered",
		"target", target.UTC().Format(time.RFC3339),
		"valid_time", validTime.UTC().Format(time.RFC3339),
		"samples", len(samples),
	)
	return overlay, nil
}

const (
	// precipDryThreshold is the mm/h below which a pixel is treated as dry and
	// left fully transparent — most of the map is dry, so this keeps the overlay
	// showing only where it actually rains.
	precipDryThreshold = 0.1
	// precipOpaqueThreshold is the mm/h at and above which intensity no longer
	// increases alpha; heavier rain just shifts colour.
	precipOpaqueThreshold = 1.0
	// precipMinAlphaFrac is the fraction of the coverage alpha applied to the
	// lightest visible rain, ramping to full at precipOpaqueThreshold.
	precipMinAlphaFrac = 0.45
)

// precipColorStops maps precipitation rate (mm/h) to a radar-style colour ramp.
var precipColorStops = []struct {
	rate float64
	rgb  color.NRGBA
}{
	{rate: 0.1, rgb: color.NRGBA{R: 120, G: 180, B: 235, A: 255}},
	{rate: 0.5, rgb: color.NRGBA{R: 70, G: 110, B: 220, A: 255}},
	{rate: 1, rgb: color.NRGBA{R: 60, G: 180, B: 160, A: 255}},
	{rate: 2, rgb: color.NRGBA{R: 90, G: 200, B: 90, A: 255}},
	{rate: 5, rgb: color.NRGBA{R: 235, G: 210, B: 70, A: 255}},
	{rate: 10, rgb: color.NRGBA{R: 235, G: 150, B: 60, A: 255}},
	{rate: 20, rgb: color.NRGBA{R: 210, G: 50, B: 40, A: 255}},
	{rate: 50, rgb: color.NRGBA{R: 180, G: 40, B: 150, A: 255}},
}

// RenderPrecipitationOverlay rasterises precipitation-rate samples (mm/h) into a
// PNG, IDW-interpolated like the temperature overlay but with a precip colour
// ramp and intensity-driven alpha so dry areas stay transparent. dataTime is the
// field's valid time, carried through to the response.
func RenderPrecipitationOverlay(req MapOverlayRequest, samples []FieldSample, dataTime time.Time) (*PrecipitationOverlay, error) {
	if req.Width <= 0 || req.Height <= 0 {
		return nil, fmt.Errorf("invalid output size")
	}
	if req.MinLon >= req.MaxLon || req.MinLat >= req.MaxLat {
		return nil, fmt.Errorf("invalid bbox")
	}
	if len(samples) < overlayMinSamples {
		return nil, fmt.Errorf("not enough samples")
	}

	img := image.NewNRGBA(image.Rect(0, 0, req.Width, req.Height))
	lonSpan := req.MaxLon - req.MinLon

	for y := 0; y < req.Height; y++ {
		lat := latitudeAtRow(req.MinLat, req.MaxLat, y, req.Height)
		cosLat := math.Cos(lat * math.Pi / 180.0)
		for x := 0; x < req.Width; x++ {
			u := float64(x) / float64(maxInt(req.Width-1, 1))
			lon := req.MinLon + u*lonSpan

			rate, nearest := interpolateField(samples, lat, lon, cosLat)
			if math.IsNaN(rate) || math.IsInf(rate, 0) {
				continue
			}

			alpha := precipAlpha(rate, nearest)
			if alpha == 0 {
				continue
			}
			base := rampColorForPrecip(rate)
			img.SetNRGBA(x, y, color.NRGBA{R: base.R, G: base.G, B: base.B, A: alpha})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}

	return &PrecipitationOverlay{
		PNG:      buf.Bytes(),
		DataTime: dataTime.UTC().Truncate(time.Second),
		Layer:    "harmonie:precipitationRate",
	}, nil
}

// interpolateField is the FieldSample counterpart of interpolateTemperature:
// inverse-distance-weighted value plus the squared distance to the nearest
// sample (for coverage alpha).
func interpolateField(samples []FieldSample, lat, lon, cosLat float64) (float64, float64) {
	var (
		sumW      float64
		sumVal    float64
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
		sumVal += w * s.Value
	}
	if sumW == 0 {
		return math.NaN(), nearestSq
	}
	return sumVal / sumW, nearestSq
}

func precipAlpha(rate, nearestDistSq float64) uint8 {
	if rate < precipDryThreshold {
		return 0
	}
	cov := alphaForCoverage(nearestDistSq)
	if cov == 0 {
		return 0
	}
	intensity := 1.0
	if rate < precipOpaqueThreshold {
		intensity = (rate - precipDryThreshold) / (precipOpaqueThreshold - precipDryThreshold)
	}
	frac := precipMinAlphaFrac + (1-precipMinAlphaFrac)*intensity
	a := float64(cov) * frac
	if a < 1 {
		return 0
	}
	if a > 255 {
		return 255
	}
	return uint8(a + 0.5)
}

func rampColorForPrecip(rate float64) color.NRGBA {
	if rate <= precipColorStops[0].rate {
		return precipColorStops[0].rgb
	}
	lastIdx := len(precipColorStops) - 1
	if rate >= precipColorStops[lastIdx].rate {
		return precipColorStops[lastIdx].rgb
	}

	for i := 0; i < lastIdx; i++ {
		a := precipColorStops[i]
		b := precipColorStops[i+1]
		if rate > b.rate {
			continue
		}

		t := (rate - a.rate) / (b.rate - a.rate)
		return color.NRGBA{
			R: uint8(float64(a.rgb.R) + (float64(b.rgb.R)-float64(a.rgb.R))*t),
			G: uint8(float64(a.rgb.G) + (float64(b.rgb.G)-float64(a.rgb.G))*t),
			B: uint8(float64(a.rgb.B) + (float64(b.rgb.B)-float64(a.rgb.B))*t),
			A: 255,
		}
	}

	return precipColorStops[lastIdx].rgb
}
