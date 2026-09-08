package weather

import (
	"math"
	"testing"
)

func TestFeelsLike(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		name                      string
		temp, wind, humidity, rad *float64
		want                      float64
	}{
		// Helsinki Kaisaniemi 2026-09-08 05:10 UTC; FMI's app showed 12.
		{"windy mild morning", f(15.2), f(5.9), f(93), nil, 12.25},
		{"calm", f(15.2), f(0), f(50), nil, 15.2},
		{"missing humidity is neutral", f(20), f(3), nil, nil, 18.29},
		{"humid summer day", f(25), f(2), f(80), nil, 26.47},
		{"winter wind", f(-5), f(5), f(80), nil, -10.65},
		{"sunny and calm", f(20), f(0), f(50), f(800), 23.67},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FeelsLike(c.temp, c.wind, c.humidity, c.rad)
			if got == nil {
				t.Fatal("got nil")
			}
			if math.Abs(*got-c.want) > 0.01 {
				t.Errorf("FeelsLike = %.2f, want %.2f", *got, c.want)
			}
		})
	}
	if got := FeelsLike(f(10), nil, nil, nil); got == nil || *got != 10 {
		t.Errorf("missing wind should return air temperature, got %v", got)
	}
	if got := FeelsLike(nil, f(3), nil, nil); got != nil {
		t.Errorf("missing temperature should return nil, got %v", *got)
	}
}
