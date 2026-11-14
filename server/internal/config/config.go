package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"slices"
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
	RequestTimeout         time.Duration

	ObservationInterval time.Duration
	ForecastCacheTTL    time.Duration

	FMIDownloadURL  string
	GribsvcURL      string
	GRIBDataDir     string
	GribFilename    string
	GribProducer    string
	GribParams      string
	GribTempParam   string
	GribPrecipParam string
	GribStep        int
	GribPrecipStep  int
	GribBBox        string
	GribInterval    time.Duration
	GribFetchEnable bool

	RadarWMSURL      string
	RadarLayer       string
	RadarBBox        string
	RadarWidth       int
	RadarHeight      int
	RadarGridStep    int
	RadarFrameSpan   time.Duration
	RadarFetchEnable bool
}

// Load reads the configuration from the environment. It returns every
// invalid setting it finds in one error rather than falling back to defaults.
func Load() (Config, error) {
	var l loader
	cfg := Config{
		Port:                   l.port("PORT", "8080"),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://weather:weather@localhost:5432/weather?sslmode=disable"),
		FMIBaseURL:             l.url("FMI_BASE_URL", "https://opendata.fmi.fi/wfs"),
		FMIAPIKey:              getEnv("FMI_API_KEY", ""),
		FMITimeseriesURL:       l.url("FMI_TIMESERIES_URL", "https://data.fmi.fi"),
		FMIWMSBaseURL:          l.url("FMI_WMS_BASE_URL", "https://data.fmi.fi"),
		FMIPrecipObsLayer:      getEnv("FMI_PRECIP_OBS_LAYER", "weatherapp:finland:precipitationObservations5min"),
		FMIPrecipFcstLayer:     getEnv("FMI_PRECIP_FCST_LAYER", "weatherapp:finland:precipitationForecast5min"),
		FMIPrecipStyle:         getEnv("FMI_PRECIP_STYLE", "Mobile_dark"),
		ClientSecrets:          l.clientSecrets("CLIENT_SECRETS"),
		RequestSignatureMaxAge: time.Duration(l.positiveInt("REQUEST_SIGNATURE_MAX_AGE_SECONDS", 300)) * time.Second,
		RequestTimeout:         time.Duration(l.positiveInt("REQUEST_TIMEOUT_SECONDS", 12)) * time.Second,

		ObservationInterval: time.Duration(l.positiveInt("OBSERVATION_INTERVAL_MINUTES", 10)) * time.Minute,
		ForecastCacheTTL:    time.Duration(l.positiveInt("FORECAST_CACHE_TTL_MINUTES", 10)) * time.Minute,

		FMIDownloadURL:  l.url("FMI_DOWNLOAD_URL", "https://opendata.fmi.fi/download"),
		GribsvcURL:      l.url("GRIBSVC_URL", "http://gribsvc:9090"),
		GRIBDataDir:     getEnv("GRIB_DATA_DIR", "/data"),
		GribFilename:    getEnv("GRIB_FILENAME", "harmonie_surface.grib2"),
		GribProducer:    getEnv("GRIB_PRODUCER", "harmonie_scandinavia_surface"),
		GribParams:      getEnv("GRIB_PARAMS", "temperature,precipitation1h"),
		GribTempParam:   getEnv("GRIB_TEMP_PARAM", "2t"),
		GribPrecipParam: getEnv("GRIB_PRECIP_PARAM", "prate"),
		GribStep:        l.positiveInt("GRIB_STEP", 2),
		GribPrecipStep:  l.positiveInt("GRIB_PRECIP_STEP", 2),
		// Slightly padded past the fixed map render extent (FINLAND in the web
		// client) so GRIB overlays span the same canvas as the radar PNGs.
		GribBBox:        l.bbox("GRIB_BBOX", "10,56.5,37.6,71.5"),
		GribInterval:    time.Duration(l.positiveInt("GRIB_INTERVAL_MINUTES", 60)) * time.Minute,
		GribFetchEnable: l.boolean("GRIB_FETCH_ENABLE"),

		// Keyless radar rain-rate composites from FMI's open WMS, fetched as
		// EPSG:4326 GeoTIFF data grids (not styled tiles). The bbox matches the
		// radar network's useful coverage; width/height set the stored raster
		// resolution (~1.3km, close to the 1km radar native so the grid render
		// matches the sharpness of FMI's own PNGs), RADAR_GRID_STEP the
		// extraction stride on top.
		RadarWMSURL:      l.url("RADAR_WMS_URL", "https://openwms.fmi.fi/geoserver/Radar/wms"),
		RadarLayer:       getEnv("RADAR_LAYER", "Radar:suomi_rr_eureffin"),
		RadarBBox:        l.bbox("RADAR_BBOX", "19,59,32,71.5"),
		RadarWidth:       l.positiveInt("RADAR_WIDTH", 512),
		RadarHeight:      l.positiveInt("RADAR_HEIGHT", 1024),
		RadarGridStep:    l.positiveInt("RADAR_GRID_STEP", 1),
		RadarFrameSpan:   time.Duration(l.positiveInt("RADAR_FRAME_SPAN_MINUTES", 70)) * time.Minute,
		RadarFetchEnable: l.boolean("RADAR_FETCH_ENABLE"),
	}
	if len(l.errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %w", errors.Join(l.errs...))
	}
	return cfg, nil
}

// LogValue renders the configuration for the startup log with credentials
// left out.
func (c Config) LogValue() slog.Value {
	clients := make([]string, 0, len(c.ClientSecrets))
	for id := range c.ClientSecrets {
		clients = append(clients, id)
	}
	slices.Sort(clients)

	return slog.GroupValue(
		slog.String("port", c.Port),
		slog.String("database", redactURL(c.DatabaseURL)),
		slog.String("fmi_base_url", c.FMIBaseURL),
		slog.Bool("fmi_api_key_set", c.FMIAPIKey != ""),
		slog.Any("clients", clients),
		slog.String("request_timeout", c.RequestTimeout.String()),
		slog.String("observation_interval", c.ObservationInterval.String()),
		slog.String("forecast_cache_ttl", c.ForecastCacheTTL.String()),
		slog.String("gribsvc_url", c.GribsvcURL),
		slog.Bool("grib_fetch", c.GribFetchEnable),
		slog.String("grib_interval", c.GribInterval.String()),
		slog.Bool("radar_fetch", c.RadarFetchEnable),
	)
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "set"
	}
	return u.Redacted()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type loader struct {
	errs []error
}

func (l *loader) fail(key, format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf("%s: %s", key, fmt.Sprintf(format, args...)))
}

func (l *loader) positiveInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		l.fail(key, "%q is not a positive integer", raw)
		return fallback
	}
	return v
}

func (l *loader) port(key, fallback string) string {
	raw := getEnv(key, fallback)
	if v, err := strconv.Atoi(raw); err != nil || v < 1 || v > 65535 {
		l.fail(key, "%q is not a port number", raw)
	}
	return raw
}

func (l *loader) boolean(key string) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		l.fail(key, "%q is not a boolean (use 1 or 0)", raw)
	}
	return v
}

func (l *loader) url(key, fallback string) string {
	raw := getEnv(key, fallback)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		l.fail(key, "%q is not an http(s) URL", raw)
	}
	return raw
}

func (l *loader) bbox(key, fallback string) string {
	raw := getEnv(key, fallback)
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		l.fail(key, "%q is not minLon,minLat,maxLon,maxLat", raw)
		return raw
	}
	var v [4]float64
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || f != f {
			l.fail(key, "%q is not minLon,minLat,maxLon,maxLat", raw)
			return raw
		}
		v[i] = f
	}
	if v[0] >= v[2] || v[1] >= v[3] || v[0] < -180 || v[2] > 180 || v[1] < -90 || v[3] > 90 {
		l.fail(key, "%q does not describe a box in degrees", raw)
	}
	return raw
}

func (l *loader) clientSecrets(key string) map[string]string {
	out := map[string]string{}
	for i, entry := range strings.Split(os.Getenv(key), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		clientID, secret, ok := strings.Cut(entry, ":")
		clientID, secret = strings.TrimSpace(clientID), strings.TrimSpace(secret)
		if !ok || clientID == "" || secret == "" {
			l.fail(key, "entry %d is not id:secret", i+1)
			continue
		}
		out[clientID] = secret
	}
	if len(out) == 0 {
		l.fail(key, "names no client, so every /v1 request would be rejected")
	}
	return out
}
