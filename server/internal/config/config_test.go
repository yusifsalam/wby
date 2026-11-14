package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PORT", "DATABASE_URL", "FMI_BASE_URL", "FMI_API_KEY", "GRIB_FETCH_ENABLE", "RADAR_FETCH_ENABLE",
		"GRIB_STEP", "GRIB_BBOX", "RADAR_BBOX", "REQUEST_TIMEOUT_SECONDS", "OBSERVATION_INTERVAL_MINUTES",
		"FORECAST_CACHE_TTL_MINUTES", "GRIB_INTERVAL_MINUTES", "GRIBSVC_URL",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("CLIENT_SECRETS", "web:dev-secret, ios : other-secret")
}

func TestLoadDefaults(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8080" || cfg.ObservationInterval != 10*time.Minute || cfg.ForecastCacheTTL != 10*time.Minute ||
		cfg.RequestTimeout != 12*time.Second || cfg.GribInterval != time.Hour {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.GribFetchEnable || cfg.RadarFetchEnable {
		t.Error("fetch loops should be off unless enabled")
	}
	if cfg.ClientSecrets["web"] != "dev-secret" || cfg.ClientSecrets["ios"] != "other-secret" {
		t.Errorf("client secrets = %v", cfg.ClientSecrets)
	}
}

func TestLoadAcceptsBooleanSpellings(t *testing.T) {
	for raw, want := range map[string]bool{"1": true, "true": true, "0": false, "false": false} {
		setValidEnv(t)
		t.Setenv("GRIB_FETCH_ENABLE", raw)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("GRIB_FETCH_ENABLE=%q: %v", raw, err)
		}
		if cfg.GribFetchEnable != want {
			t.Errorf("GRIB_FETCH_ENABLE=%q loaded as %v, want %v", raw, cfg.GribFetchEnable, want)
		}
	}
}

func TestLoadRejectsInvalidSettings(t *testing.T) {
	cases := map[string]string{
		"PORT":                         "eighty",
		"GRIB_STEP":                    "0",
		"GRIB_INTERVAL_MINUTES":        "hourly",
		"REQUEST_TIMEOUT_SECONDS":      "-5",
		"GRIB_FETCH_ENABLE":            "yes please",
		"FMI_BASE_URL":                 "opendata.fmi.fi/wfs",
		"GRIBSVC_URL":                  "ftp://gribsvc",
		"GRIB_BBOX":                    "10,56.5,37.6",
		"RADAR_BBOX":                   "32,59,19,71.5",
		"OBSERVATION_INTERVAL_MINUTES": "ten",
	}
	for key, value := range cases {
		setValidEnv(t)
		t.Setenv(key, value)
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s=%q: err = %v, want one naming the variable", key, value, err)
		}
	}
}

func TestLoadRejectsUnusableClientSecrets(t *testing.T) {
	for _, raw := range []string{"", "just-a-secret", "web:", ":secret", "web:ok,broken"} {
		setValidEnv(t)
		t.Setenv("CLIENT_SECRETS", raw)
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "CLIENT_SECRETS") {
			t.Errorf("CLIENT_SECRETS=%q: err = %v, want a CLIENT_SECRETS error", raw, err)
			continue
		}
		if raw != "" && strings.Contains(err.Error(), "secret") && strings.Contains(err.Error(), "just-a-secret") {
			t.Errorf("error repeats the configured value: %v", err)
		}
	}
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	setValidEnv(t)
	t.Setenv("PORT", "eighty")
	t.Setenv("GRIB_STEP", "none")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORT") || !strings.Contains(err.Error(), "GRIB_STEP") {
		t.Fatalf("err = %v, want both PORT and GRIB_STEP named", err)
	}
}

func TestLogValueLeavesOutCredentials(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DATABASE_URL", "postgres://weather:db-password@db:5432/weather?sslmode=disable")
	t.Setenv("FMI_API_KEY", "fmi-key-value")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	slog.New(slog.NewJSONHandler(&out, nil)).Info("configuration loaded", "config", cfg)
	logged := out.String()
	for _, secret := range []string{"db-password", "fmi-key-value", "dev-secret", "other-secret"} {
		if strings.Contains(logged, secret) {
			t.Errorf("startup log contains %q: %s", secret, logged)
		}
	}
	for _, want := range []string{`"fmi_api_key_set":true`, `"clients":["ios","web"]`, "db:5432"} {
		if !strings.Contains(logged, want) {
			t.Errorf("startup log lacks %s: %s", want, logged)
		}
	}
}
