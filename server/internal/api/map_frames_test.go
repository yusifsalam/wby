package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildMapFrames(t *testing.T) {
	now := time.Date(2026, 7, 2, 5, 37, 12, 0, time.UTC)
	frames := buildMapFrames(now)

	if got := len(frames.Temperature.Times); got != temperatureFrameHours+1 {
		t.Fatalf("expected %d temperature frames, got %d", temperatureFrameHours+1, got)
	}
	if frames.Temperature.NowIndex != 0 {
		t.Fatalf("expected temperature now_index 0, got %d", frames.Temperature.NowIndex)
	}
	if frames.Temperature.Times[0] != "2026-07-02T05:00:00Z" {
		t.Fatalf("expected first temperature frame 05:00Z, got %s", frames.Temperature.Times[0])
	}
	wantLast := time.Date(2026, 7, 2, 5, 0, 0, 0, time.UTC).
		Add(temperatureFrameHours * time.Hour).Format(time.RFC3339)
	if last := frames.Temperature.Times[temperatureFrameHours]; last != wantLast {
		t.Fatalf("expected last temperature frame %s, got %s", wantLast, last)
	}

	if got := len(frames.Precipitation.Times); got != 25 {
		t.Fatalf("expected 25 precipitation frames, got %d", got)
	}
	if frames.Precipitation.NowIndex != 12 {
		t.Fatalf("expected precipitation now_index 12, got %d", frames.Precipitation.NowIndex)
	}
	if frames.Precipitation.Times[12] != "2026-07-02T05:35:00Z" {
		t.Fatalf("expected now precipitation frame 05:35Z, got %s", frames.Precipitation.Times[12])
	}
	if frames.Precipitation.Times[0] != "2026-07-02T04:35:00Z" {
		t.Fatalf("expected first precipitation frame 04:35Z, got %s", frames.Precipitation.Times[0])
	}
	if frames.Precipitation.Times[24] != "2026-07-02T06:35:00Z" {
		t.Fatalf("expected last precipitation frame 06:35Z, got %s", frames.Precipitation.Times[24])
	}

	if got := len(frames.Precipitation12h.Times); got != precipitation12hFrameHours+1 {
		t.Fatalf("expected %d precipitation12h frames, got %d", precipitation12hFrameHours+1, got)
	}
	if frames.Precipitation12h.NowIndex != 0 {
		t.Fatalf("expected precipitation12h now_index 0, got %d", frames.Precipitation12h.NowIndex)
	}
	if frames.Precipitation12h.StepSeconds != 3600 {
		t.Fatalf("expected precipitation12h step 3600s, got %d", frames.Precipitation12h.StepSeconds)
	}
	if frames.Precipitation12h.Times[0] != "2026-07-02T05:00:00Z" {
		t.Fatalf("expected first precipitation12h frame 05:00Z, got %s", frames.Precipitation12h.Times[0])
	}
	if frames.Precipitation12h.Times[12] != "2026-07-02T17:00:00Z" {
		t.Fatalf("expected last precipitation12h frame 17:00Z, got %s", frames.Precipitation12h.Times[12])
	}
}

func TestGetMapFrames(t *testing.T) {
	h := NewHandler(fakeWeatherService{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/map/frames", nil)

	h.getMapFrames(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var resp mapFramesJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(resp.Temperature.Times) == 0 || len(resp.Precipitation.Times) == 0 {
		t.Fatalf("expected non-empty frame lists, got %+v", resp)
	}
}
