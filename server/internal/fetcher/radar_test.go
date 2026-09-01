package fetcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wby/internal/weather"
)

func TestRadarSidecarJSON(t *testing.T) {
	job := RadarJob{BBox: "19,59,32,71.5"}
	at := time.Date(2026, 9, 1, 16, 5, 0, 0, time.UTC)

	raw, err := radarSidecarJSON(job, at)
	if err != nil {
		t.Fatalf("radarSidecarJSON: %v", err)
	}

	var meta struct {
		Param  string    `json:"param"`
		Time   string    `json:"time"`
		BBox   []float64 `json:"bbox"`
		Scale  float64   `json:"scale"`
		Nodata int       `json:"nodata"`
		Units  string    `json:"units"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal sidecar: %v", err)
	}
	if meta.Param != "rr" || meta.Units != "mm/h" {
		t.Fatalf("param/units = %q/%q", meta.Param, meta.Units)
	}
	if meta.Time != "2026-09-01T16:05:00Z" {
		t.Fatalf("time = %q", meta.Time)
	}
	if len(meta.BBox) != 4 || meta.BBox[0] != 19 || meta.BBox[3] != 71.5 {
		t.Fatalf("bbox = %v", meta.BBox)
	}
	if meta.Scale != 0.01 || meta.Nodata != 65535 {
		t.Fatalf("scale/nodata = %v/%v", meta.Scale, meta.Nodata)
	}
}

func TestRadarSidecarJSONRejectsBadBBox(t *testing.T) {
	if _, err := radarSidecarJSON(RadarJob{BBox: "19,59,32"}, time.Now()); err == nil {
		t.Fatal("want error for 3-element bbox")
	}
}

func TestPruneRadarFrames(t *testing.T) {
	dir := t.TempDir()
	f := &Fetcher{}
	job := RadarJob{DataDir: dir, Span: time.Hour}

	old := time.Now().UTC().Add(-3 * time.Hour)
	fresh := time.Now().UTC().Truncate(radarFrameStep)

	oldTif := weather.RadarFrameFile(old)
	oldJSON := oldTif[:len(oldTif)-4] + ".json"
	freshTif := weather.RadarFrameFile(fresh)
	unrelated := "harmonie_surface.grib2"

	for _, name := range []string{oldTif, oldJSON, freshTif, unrelated} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	f.pruneRadarFrames(job)

	for _, gone := range []string{oldTif, oldJSON} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Fatalf("%s should have been pruned", gone)
		}
	}
	for _, kept := range []string{freshTif, unrelated} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Fatalf("%s should have been kept: %v", kept, err)
		}
	}
}
