package weather

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeWMS struct {
	published time.Time
	delay     time.Duration
	calls     atomic.Int32
}

func (f *fakeWMS) FetchWMSTile(ctx context.Context, req WMSTileRequest) ([]byte, error) {
	f.calls.Add(1)
	time.Sleep(f.delay)
	if req.Time.After(f.published) {
		return nil, errors.New("InvalidDimensionValue: Invalid time requested!")
	}
	return []byte("png"), nil
}

func newPrecipTileTestService(wms WMSTileFetcher) *Service {
	return &Service{
		wms:            wms,
		precipObsLayer: "observation",
		precipCache:    NewCache[*PrecipitationOverlay](30 * time.Minute),
	}
}

func precipTileTTL(t *testing.T, s *Service) time.Duration {
	t.Helper()
	s.precipCache.mu.RLock()
	defer s.precipCache.mu.RUnlock()
	if len(s.precipCache.m) != 1 {
		t.Fatalf("cache holds %d tiles, want 1", len(s.precipCache.m))
	}
	for _, entry := range s.precipCache.m {
		return time.Until(entry.expiresAt)
	}
	return 0
}

func TestPrecipitationTileFallbackFrameIsCachedBriefly(t *testing.T) {
	const step = 5 * time.Minute
	target := time.Now().UTC().Truncate(step).Add(-step)
	req := PrecipitationOverlayRequest{Time: target, MapOverlayRequest: MapOverlayRequest{MinLon: 19, MinLat: 59, MaxLon: 32, MaxLat: 71, Width: 256, Height: 256}}

	lagging := newPrecipTileTestService(&fakeWMS{published: target.Add(-2 * step)})
	overlay, err := lagging.GetPrecipitationOverlay(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !overlay.DataTime.Equal(target.Add(-2 * step)) {
		t.Fatalf("DataTime = %v, want the newest published frame %v", overlay.DataTime, target.Add(-2*step))
	}
	if ttl := precipTileTTL(t, lagging); ttl > upstreamRetryAfter {
		t.Errorf("an earlier frame standing in for the requested one is cached for %v, want at most %v", ttl, upstreamRetryAfter)
	}

	current := newPrecipTileTestService(&fakeWMS{published: target})
	if _, err := current.GetPrecipitationOverlay(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if ttl := precipTileTTL(t, current); ttl < 25*time.Minute {
		t.Errorf("the requested frame is cached for %v, want the full cache TTL", ttl)
	}
}

func TestConcurrentPrecipitationTileRequestsFetchOnce(t *testing.T) {
	const step = 5 * time.Minute
	target := time.Now().UTC().Truncate(step).Add(-step)
	wms := &fakeWMS{published: target, delay: 50 * time.Millisecond}
	svc := newPrecipTileTestService(wms)
	req := PrecipitationOverlayRequest{Time: target, MapOverlayRequest: MapOverlayRequest{MinLon: 19, MinLat: 59, MaxLon: 32, MaxLat: 71, Width: 256, Height: 256}}

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.GetPrecipitationOverlay(context.Background(), req); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if n := wms.calls.Load(); n != 1 {
		t.Errorf("tile fetched %d times, want 1", n)
	}
}
