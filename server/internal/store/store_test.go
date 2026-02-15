package store

import (
	"context"
	"os"
	"testing"

	"wby/internal/weather"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://weather:weather@localhost:5432/weather?sslmode=disable"
	}
	s, err := New(context.Background(), dsn)
	if err != nil {
		t.Skipf("database not available: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertStations(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	stations := []weather.Station{
		{FMISID: 100971, Name: "Helsinki Kaisaniemi", Lat: 60.17523, Lon: 24.94459, WMOCode: "2978"},
	}

	err := s.UpsertStations(ctx, stations)
	if err != nil {
		t.Fatal(err)
	}

	nearest, dist, err := s.NearestStation(ctx, 60.17, 24.94)
	if err != nil {
		t.Fatal(err)
	}
	if nearest.FMISID != 100971 {
		t.Errorf("expected station 100971, got %d", nearest.FMISID)
	}
	if dist > 1.0 {
		t.Errorf("expected distance < 1km, got %f", dist)
	}
}
