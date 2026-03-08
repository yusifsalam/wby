package weather

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"
)

func TestRenderTemperatureOverlay(t *testing.T) {
	req := MapOverlayRequest{
		MinLon: 24.6,
		MinLat: 60.0,
		MaxLon: 25.2,
		MaxLat: 60.5,
		Width:  180,
		Height: 120,
	}
	samples := []TemperatureSample{
		{Lat: 60.17, Lon: 24.94, Temperature: -3, ObservedAt: time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)},
		{Lat: 60.30, Lon: 24.80, Temperature: 1, ObservedAt: time.Date(2026, 3, 7, 10, 5, 0, 0, time.UTC)},
		{Lat: 60.10, Lon: 25.15, Temperature: 4, ObservedAt: time.Date(2026, 3, 7, 9, 55, 0, 0, time.UTC)},
	}

	overlay, err := RenderTemperatureOverlay(req, samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(overlay.PNG) == 0 {
		t.Fatal("expected png bytes")
	}
	if overlay.MinTemp != -3 || overlay.MaxTemp != 4 {
		t.Fatalf("unexpected min/max: %f %f", overlay.MinTemp, overlay.MaxTemp)
	}
	if !overlay.DataTime.Equal(time.Date(2026, 3, 7, 10, 5, 0, 0, time.UTC)) {
		t.Fatalf("unexpected data time: %s", overlay.DataTime)
	}

	img, err := png.Decode(bytes.NewReader(overlay.PNG))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if img.Bounds() != image.Rect(0, 0, req.Width, req.Height) {
		t.Fatalf("unexpected image bounds: %v", img.Bounds())
	}
}

func TestRenderTemperatureOverlay_NotEnoughSamples(t *testing.T) {
	req := MapOverlayRequest{MinLon: 24, MinLat: 60, MaxLon: 25, MaxLat: 61, Width: 200, Height: 100}
	_, err := RenderTemperatureOverlay(req, []TemperatureSample{
		{Lat: 60.1, Lon: 24.9, Temperature: 1},
		{Lat: 60.2, Lon: 25.0, Temperature: 2},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeTemperature_UsesFixedScaleAndClamp(t *testing.T) {
	if got := normalizeTemperature(-50); got != 0 {
		t.Fatalf("expected lower clamp to 0, got %f", got)
	}
	if got := normalizeTemperature(50); got != 1 {
		t.Fatalf("expected upper clamp to 1, got %f", got)
	}
	mid := normalizeTemperature(0)
	if mid < 0.49 || mid > 0.51 {
		t.Fatalf("expected zero to map near midpoint, got %f", mid)
	}
}

func TestRenderTemperatureOverlay_UsesAbsoluteColorScale(t *testing.T) {
	req := MapOverlayRequest{
		MinLon: 24.0,
		MinLat: 60.0,
		MaxLon: 24.4,
		MaxLat: 60.4,
		Width:  3,
		Height: 3,
	}
	centerLat := 60.2
	centerLon := 24.2

	baseSamples := []TemperatureSample{
		{Lat: centerLat, Lon: centerLon, Temperature: 5, ObservedAt: time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)},
		{Lat: 60.39, Lon: 24.01, Temperature: 4, ObservedAt: time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)},
		{Lat: 60.01, Lon: 24.39, Temperature: 6, ObservedAt: time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)},
	}

	outlierSamples := []TemperatureSample{
		{Lat: centerLat, Lon: centerLon, Temperature: 5, ObservedAt: time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)},
		{Lat: 60.39, Lon: 24.01, Temperature: -40, ObservedAt: time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)},
		{Lat: 60.01, Lon: 24.39, Temperature: 40, ObservedAt: time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)},
	}

	baseOverlay, err := RenderTemperatureOverlay(req, baseSamples)
	if err != nil {
		t.Fatalf("render base overlay: %v", err)
	}
	outlierOverlay, err := RenderTemperatureOverlay(req, outlierSamples)
	if err != nil {
		t.Fatalf("render outlier overlay: %v", err)
	}

	baseCenter := decodeCenterPixel(t, baseOverlay.PNG)
	outlierCenter := decodeCenterPixel(t, outlierOverlay.PNG)
	if !roughlyEqualColor(baseCenter, outlierCenter, 1) {
		t.Fatalf("expected near-identical center color for same temperature with absolute scale, got base=%v outlier=%v", baseCenter, outlierCenter)
	}
}

func TestRenderTemperatureOverlay_MasksAreasFarFromStations(t *testing.T) {
	req := MapOverlayRequest{
		MinLon: 0,
		MinLat: 0,
		MaxLon: 20,
		MaxLat: 20,
		Width:  21,
		Height: 21,
	}

	samples := []TemperatureSample{
		{Lat: 10.0, Lon: 10.0, Temperature: 4, ObservedAt: time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)},
		{Lat: 10.2, Lon: 10.1, Temperature: 4.5, ObservedAt: time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)},
		{Lat: 9.8, Lon: 9.9, Temperature: 3.5, ObservedAt: time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)},
	}

	overlay, err := RenderTemperatureOverlay(req, samples)
	if err != nil {
		t.Fatalf("render overlay: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(overlay.PNG))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}

	center := color.NRGBAModel.Convert(img.At(10, 10)).(color.NRGBA)
	if center.A == 0 {
		t.Fatalf("expected data overlay at center pixel, got alpha=0")
	}

	corner := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
	if corner.A != 0 {
		t.Fatalf("expected far corner to be transparent, got alpha=%d", corner.A)
	}
}

func TestLatitudeAtRow_UsesMercatorProjection(t *testing.T) {
	minLat := 33.0
	maxLat := 71.0
	height := 1201

	top := latitudeAtRow(minLat, maxLat, 0, height)
	bottom := latitudeAtRow(minLat, maxLat, height-1, height)
	mid := latitudeAtRow(minLat, maxLat, height/2, height)

	if top < maxLat-0.01 || top > maxLat+0.01 {
		t.Fatalf("unexpected top latitude: %f", top)
	}
	if bottom < minLat-0.01 || bottom > minLat+0.01 {
		t.Fatalf("unexpected bottom latitude: %f", bottom)
	}

	linearMid := (minLat + maxLat) / 2
	if mid <= linearMid {
		t.Fatalf("expected mercator midpoint latitude > linear midpoint (%f), got %f", linearMid, mid)
	}
}

func decodeCenterPixel(t *testing.T, pngBytes []byte) color.NRGBA {
	t.Helper()

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	bounds := img.Bounds()
	x := bounds.Min.X + bounds.Dx()/2
	y := bounds.Min.Y + bounds.Dy()/2
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

func roughlyEqualColor(a, b color.NRGBA, tolerance uint8) bool {
	return channelDiff(a.R, b.R) <= tolerance &&
		channelDiff(a.G, b.G) <= tolerance &&
		channelDiff(a.B, b.B) <= tolerance &&
		channelDiff(a.A, b.A) <= tolerance
}

func channelDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
