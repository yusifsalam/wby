package weather

import (
	"bytes"
	"image"
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
