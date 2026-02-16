package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	FMIBaseURL       string
	FMIAPIKey        string
	FMITimeseriesURL string
}

func Load() Config {
	return Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://weather:weather@localhost:5432/weather?sslmode=disable"),
		FMIBaseURL:       getEnv("FMI_BASE_URL", "https://opendata.fmi.fi/wfs"),
		FMIAPIKey:        getEnv("FMI_API_KEY", ""),
		FMITimeseriesURL: getEnv("FMI_TIMESERIES_URL", "https://data.fmi.fi"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
