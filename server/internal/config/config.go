package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                   string
	DatabaseURL            string
	FMIBaseURL             string
	FMIAPIKey              string
	FMITimeseriesURL       string
	ClientSecrets          map[string]string
	RequestSignatureMaxAge time.Duration
}

func Load() Config {
	return Config{
		Port:                   getEnv("PORT", "8080"),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://weather:weather@localhost:5432/weather?sslmode=disable"),
		FMIBaseURL:             getEnv("FMI_BASE_URL", "https://opendata.fmi.fi/wfs"),
		FMIAPIKey:              getEnv("FMI_API_KEY", ""),
		FMITimeseriesURL:       getEnv("FMI_TIMESERIES_URL", "https://data.fmi.fi"),
		ClientSecrets:          parseClientSecrets(getEnv("CLIENT_SECRETS", "")),
		RequestSignatureMaxAge: time.Duration(getEnvInt("REQUEST_SIGNATURE_MAX_AGE_SECONDS", 300)) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func parseClientSecrets(raw string) map[string]string {
	out := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			continue
		}

		clientID := strings.TrimSpace(parts[0])
		secret := strings.TrimSpace(parts[1])
		if clientID == "" || secret == "" {
			continue
		}
		out[clientID] = secret
	}
	return out
}
