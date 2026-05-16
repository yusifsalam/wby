package weather

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// ErrPrecipitationDisabled is returned when the WMS client or layer config
// is missing, so the precipitation feature can't be served.
var ErrPrecipitationDisabled = errors.New("precipitation overlay disabled")

// PrecipitationOverlay is the rendered precipitation tile + metadata returned
// to clients. Mirrors the shape of TemperatureOverlay so the iOS overlay path
// can reuse its existing rendering.
type PrecipitationOverlay struct {
	PNG      []byte
	DataTime time.Time // the requested timestamp (snapped to the hour)
	Layer    string    // which WMS layer was used (observation vs forecast)
}

// PrecipitationOverlayRequest extends MapOverlayRequest with a target time.
type PrecipitationOverlayRequest struct {
	MapOverlayRequest
	Time time.Time
}

func (s *Service) GetPrecipitationOverlay(ctx context.Context, req PrecipitationOverlayRequest) (*PrecipitationOverlay, error) {
	if s.wms == nil || (s.precipObsLayer == "" && s.precipFcstLayer == "") {
		return nil, ErrPrecipitationDisabled
	}

	const precipStep = 5 * time.Minute
	target := req.Time.UTC().Truncate(precipStep)
	if target.IsZero() {
		target = time.Now().UTC().Truncate(precipStep)
	}

	layer := s.pickPrecipLayer(target)
	if layer == "" {
		return nil, ErrPrecipitationDisabled
	}

	cacheKey := precipCacheKey(layer, target, req.MapOverlayRequest)
	if cached, ok := s.precipCache.Get(cacheKey); ok {
		return cached, nil
	}

	// FMI's radar/forecast tiles for the most recent 5-min mark may not be
	// published yet at the boundary. If the upstream errors, retry up to
	// `precipFallbackSteps` earlier steps so we still return *something*.
	const precipFallbackSteps = 3
	var lastErr error
	attemptTarget := target
	for attempt := 0; attempt < precipFallbackSteps; attempt++ {
		tile, err := s.wms.FetchWMSTile(ctx, WMSTileRequest{
			Layer:  layer,
			Style:  s.precipStyle,
			Time:   attemptTarget,
			MinLon: req.MinLon,
			MinLat: req.MinLat,
			MaxLon: req.MaxLon,
			MaxLat: req.MaxLat,
			Width:  req.Width,
			Height: req.Height,
		})
		if err == nil {
			overlay := &PrecipitationOverlay{
				PNG:      tile,
				DataTime: attemptTarget,
				Layer:    layer,
			}
			s.precipCache.Set(cacheKey, overlay)
			if attempt > 0 {
				slog.Info("precipitation tile fallback succeeded", "layer", layer,
					"requested", target, "served", attemptTarget, "attempts", attempt+1)
			}
			return overlay, nil
		}
		lastErr = err
		// Only retry for observation-class targets (live or recent past). Forecast
		// frames are time-specific so an earlier step would be wrong content.
		if layer != s.precipObsLayer {
			break
		}
		attemptTarget = attemptTarget.Add(-precipStep)
	}
	if !errors.Is(lastErr, context.Canceled) && !errors.Is(lastErr, context.DeadlineExceeded) {
		slog.Warn("precipitation tile fetch failed", "err", lastErr, "layer", layer, "time", target)
	}
	return nil, fmt.Errorf("fetch precipitation tile: %w", lastErr)
}

func (s *Service) pickPrecipLayer(target time.Time) string {
	now := time.Now().UTC().Truncate(5 * time.Minute)
	// Use the forecast layer for any time strictly in the future. The current
	// step is treated as observation since the radar composite for the current
	// 5-min mark is normally already published.
	if target.After(now) {
		if s.precipFcstLayer != "" {
			return s.precipFcstLayer
		}
		return s.precipObsLayer
	}
	if s.precipObsLayer != "" {
		return s.precipObsLayer
	}
	return s.precipFcstLayer
}

func precipCacheKey(layer string, t time.Time, r MapOverlayRequest) string {
	var b strings.Builder
	b.WriteString(layer)
	b.WriteByte('|')
	b.WriteString(t.UTC().Format(time.RFC3339))
	b.WriteByte('|')
	b.WriteString(strconv.FormatFloat(r.MinLon, 'f', 4, 64))
	b.WriteByte(',')
	b.WriteString(strconv.FormatFloat(r.MinLat, 'f', 4, 64))
	b.WriteByte(',')
	b.WriteString(strconv.FormatFloat(r.MaxLon, 'f', 4, 64))
	b.WriteByte(',')
	b.WriteString(strconv.FormatFloat(r.MaxLat, 'f', 4, 64))
	b.WriteByte('|')
	b.WriteString(strconv.Itoa(r.Width))
	b.WriteByte('x')
	b.WriteString(strconv.Itoa(r.Height))
	return b.String()
}
