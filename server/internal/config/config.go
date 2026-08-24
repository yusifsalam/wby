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
	FMIWMSBaseURL          string
	FMIPrecipObsLayer      string
	FMIPrecipFcstLayer     string
	FMIPrecipStyle         string
	ClientSecrets          map[string]string
	RequestSignatureMaxAge time.Duration

	FMIDownloadURL  string
	GribsvcURL      string
	GRIBDataDir     string
	GribFilename    string
	GribProducer    string
	GribParams      string
	GribTempParam   string
	GribPrecipParam string
	GribStep        int
	GribBBox        string
	GribInterval    time.Duration
	GribFetchEnable bool
}

func Load() Config {
	return Config{
		Port:                   getEnv("PORT", "8080"),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://weather:weather@localhost:5432/weather?sslmode=disable"),
		FMIBaseURL:             getEnv("FMI_BASE_URL", "https://opendata.fmi.fi/wfs"),
		FMIAPIKey:              getEnv("FMI_API_KEY", ""),
		FMITimeseriesURL:       getEnv("FMI_TIMESERIES_URL", "https://data.fmi.fi"),
		FMIWMSBaseURL:          getEnv("FMI_WMS_BASE_URL", "https://data.fmi.fi"),
		FMIPrecipObsLayer:      getEnv("FMI_PRECIP_OBS_LAYER", "weatherapp:finland:precipitationObservations5min"),
		FMIPrecipFcstLayer:     getEnv("FMI_PRECIP_FCST_LAYER", "weatherapp:finland:precipitationForecast5min"),
		FMIPrecipStyle:         getEnv("FMI_PRECIP_STYLE", "Mobile_dark"),
		ClientSecrets:          parseClientSecrets(getEnv("CLIENT_SECRETS", "")),
		RequestSignatureMaxAge: time.Duration(getEnvInt("REQUEST_SIGNATURE_MAX_AGE_SECONDS", 300)) * time.Second,

		FMIDownloadURL:  getEnv("FMI_DOWNLOAD_URL", "https://opendata.fmi.fi/download"),
		GribsvcURL:      getEnv("GRIBSVC_URL", "http://gribsvc:9090"),
		GRIBDataDir:     getEnv("GRIB_DATA_DIR", "/data"),
		GribFilename:    getEnv("GRIB_FILENAME", "harmonie_surface.grib2"),
		GribProducer:    getEnv("GRIB_PRODUCER", "harmonie_scandinavia_surface"),
		GribParams:      getEnv("GRIB_PARAMS", "temperature,precipitation1h"),
		GribTempParam:   getEnv("GRIB_TEMP_PARAM", "2t"),
		GribPrecipParam: getEnv("GRIB_PRECIP_PARAM", "prate"),
		GribStep:        getEnvInt("GRIB_STEP", 8),
		// Slightly padded past the fixed map render extent (FINLAND in the web
		// client) so GRIB overlays span the same canvas as the radar PNGs.
		GribBBox:        getEnv("GRIB_BBOX", "10,56.5,37.6,71.5"),
		GribInterval:    time.Duration(getEnvInt("GRIB_INTERVAL_MINUTES", 60)) * time.Minute,
		GribFetchEnable: getEnv("GRIB_FETCH_ENABLE", "") == "1",
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
