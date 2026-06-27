package weather

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"testing"
	"time"
)

func TestRenderPrecipitationOverlay(t *testing.T) {
	req := MapOverlayRequest{
		MinLon: 24.6,
		MinLat: 60.0,
		MaxLon: 25.2,
		MaxLat: 60.5,
		Width:  180,
		Height: 120,
	}
	dataTime := time.Date(2026, 6, 27, 14, 0, 0, 0, time.UTC)
	samples := []FieldSample{
		{Lat: 60.17, Lon: 24.94, Value: 3.2, ObservedAt: dataTime},
		{Lat: 60.30, Lon: 24.80, Value: 0.0, ObservedAt: dataTime},
		{Lat: 60.10, Lon: 25.15, Value: 0.4, ObservedAt: dataTime},
	}

	overlay, err := RenderPrecipitationOverlay(req, samples, dataTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(overlay.PNG) == 0 {
		t.Fatal("expected png bytes")
	}
	if !overlay.DataTime.Equal(dataTime) {
		t.Fatalf("unexpected data time: %s", overlay.DataTime)
	}

	img, err := png.Decode(bytes.NewReader(overlay.PNG))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if img.Bounds() != image.Rect(0, 0, req.Width, req.Height) {
		t.Fatalf("unexpected image bounds: %v", img.Bounds())
	}
	// The rainy sample's corner should be painted; the dry corner should not.
	if _, _, _, a := img.At(0, req.Height-1).RGBA(); a == 0 {
		t.Error("expected the rainy gridpoint to be opaque")
	}
}

func TestPrecipAlphaDryIsTransparent(t *testing.T) {
	if a := precipAlpha(0.0, 0); a != 0 {
		t.Fatalf("dry pixel should be transparent, got alpha %d", a)
	}
	if a := precipAlpha(precipDryThreshold-0.01, 0); a != 0 {
		t.Fatalf("sub-threshold pixel should be transparent, got alpha %d", a)
	}
	if a := precipAlpha(5.0, 0); a == 0 {
		t.Fatal("heavy rain over a sample should be opaque")
	}
}

type fakePrecipSource struct {
	samples   []FieldSample
	validTime time.Time
	err       error
}

func (f fakePrecipSource) Samples(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) ([]FieldSample, time.Time, error) {
	return f.samples, f.validTime, f.err
}

func newPrecipTestService(src GribPrecipitationSource) *Service {
	return &Service{
		gribPrecip:      src,
		gribPrecipCache: NewCache[*PrecipitationOverlay](time.Minute),
	}
}

func TestGetPrecipitationForecastOverlay_Disabled(t *testing.T) {
	svc := &Service{gribPrecipCache: NewCache[*PrecipitationOverlay](time.Minute)}
	_, err := svc.GetPrecipitationForecastOverlay(context.Background(), PrecipitationOverlayRequest{
		MapOverlayRequest: MapOverlayRequest{MinLon: 24, MinLat: 60, MaxLon: 25, MaxLat: 61, Width: 100, Height: 100},
	})
	if !errors.Is(err, ErrPrecipitationDisabled) {
		t.Fatalf("expected ErrPrecipitationDisabled, got %v", err)
	}
}

func TestGetPrecipitationForecastOverlay_SoftMissOnSparse(t *testing.T) {
	svc := newPrecipTestService(fakePrecipSource{samples: []FieldSample{{Lat: 60, Lon: 25, Value: 1}}})
	_, err := svc.GetPrecipitationForecastOverlay(context.Background(), PrecipitationOverlayRequest{
		MapOverlayRequest: MapOverlayRequest{MinLon: 24, MinLat: 60, MaxLon: 25, MaxLat: 61, Width: 100, Height: 100},
	})
	if !errors.Is(err, ErrPrecipitationDisabled) {
		t.Fatalf("expected ErrPrecipitationDisabled for sparse field, got %v", err)
	}
}

func TestGetPrecipitationForecastOverlay_RendersAndCaches(t *testing.T) {
	at := time.Date(2026, 6, 27, 14, 0, 0, 0, time.UTC)
	src := fakePrecipSource{
		samples: []FieldSample{
			{Lat: 60.1, Lon: 24.9, Value: 2.0, ObservedAt: at},
			{Lat: 60.4, Lon: 25.1, Value: 0.0, ObservedAt: at},
			{Lat: 60.7, Lon: 24.6, Value: 0.6, ObservedAt: at},
		},
		validTime: at,
	}
	svc := newPrecipTestService(src)
	req := PrecipitationOverlayRequest{
		MapOverlayRequest: MapOverlayRequest{MinLon: 24, MinLat: 60, MaxLon: 25.5, MaxLat: 61, Width: 120, Height: 120},
		Time:              at,
	}
	overlay, err := svc.GetPrecipitationForecastOverlay(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(overlay.PNG) == 0 {
		t.Fatal("expected rendered png")
	}
	if overlay.Layer != precipForecastLayer {
		t.Fatalf("unexpected layer: %s", overlay.Layer)
	}
	// Second call with the same key should hit the cache (same pointer).
	overlay2, err := svc.GetPrecipitationForecastOverlay(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on cached call: %v", err)
	}
	if overlay2 != overlay {
		t.Fatal("expected cached overlay pointer on second call")
	}
}
