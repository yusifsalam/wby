package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"

	"wby/internal/api"
	"wby/internal/config"
	"wby/internal/fetcher"
	"wby/internal/fmi"
	"wby/internal/grib"
	"wby/internal/store"
	"wby/internal/weather"
)

const (
	shutdownGrace = 10 * time.Second
	// maxObservationAge is how old the newest observation may get before the
	// service reports itself not ready.
	maxObservationAge = 30 * time.Minute
	maxRadarFrameAge  = 20 * time.Minute
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.Info("configuration loaded", "config", cfg)
	if cfg.FMIAPIKey == "" {
		slog.Warn("FMI_API_KEY is not set: UV forecasts and WMS precipitation tiles are disabled")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

	fmiClient := fmi.NewClientWithWMS(cfg.FMIBaseURL, cfg.FMIAPIKey, cfg.FMITimeseriesURL, cfg.FMIWMSBaseURL, cfg.FMIDownloadURL)

	svc := weather.NewService(db, fmiClient, cfg.ForecastCacheTTL)
	svc.SetPrecipitationLayers(cfg.FMIPrecipObsLayer, cfg.FMIPrecipFcstLayer)
	svc.SetPrecipitationStyle(cfg.FMIPrecipStyle)
	svc.SetGribTemperatureSource(grib.New(cfg.GribsvcURL, cfg.GribFilename, cfg.GribTempParam, cfg.GribStep))
	svc.SetGribPrecipitationSource(grib.NewPrecipitation(cfg.GribsvcURL, cfg.GribFilename, cfg.GribPrecipParam, cfg.GribPrecipStep))
	svc.SetGribFile(filepath.Join(cfg.GRIBDataDir, cfg.GribFilename))
	if cfg.RadarFetchEnable {
		svc.SetRadarPrecipitationSource(grib.NewRadar(cfg.GribsvcURL, cfg.RadarGridStep))
	}

	f := fetcher.New(fmiClient, db)
	var loops sync.WaitGroup
	start := func(name string, loop func(context.Context)) {
		loops.Add(1)
		go func() {
			defer loops.Done()
			fetcher.Supervise(ctx, name, loop)
		}()
	}
	start("observations", func(ctx context.Context) {
		f.RunObservationLoop(ctx, cfg.ObservationInterval)
	})
	if cfg.GribFetchEnable {
		start("grib", func(ctx context.Context) {
			f.RunGribLoop(ctx, fetcher.GribJob{
				DataDir:  cfg.GRIBDataDir,
				Filename: cfg.GribFilename,
				Producer: cfg.GribProducer,
				Params:   cfg.GribParams,
				BBox:     cfg.GribBBox,
			}, cfg.GribInterval, svc.WarmGribGrids)
		})
	}
	if cfg.RadarFetchEnable {
		start("radar", func(ctx context.Context) {
			f.RunRadarLoop(ctx, fmi.NewRadarClient(cfg.RadarWMSURL, cfg.RadarLayer), fetcher.RadarJob{
				DataDir: cfg.GRIBDataDir,
				BBox:    cfg.RadarBBox,
				Width:   cfg.RadarWidth,
				Height:  cfg.RadarHeight,
				Span:    cfg.RadarFrameSpan,
				Nowcast: svc,
			}, func(ctx context.Context) {
				svc.WarmRadarGrids(ctx)
				svc.WarmNowcastGrids(ctx)
			})
		})
	}

	mux := http.NewServeMux()
	handler := api.NewHandler(svc)
	handler.SetHealthChecks(healthChecks(cfg, db, f, svc)...)
	handler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: api.Chain(mux,
			api.AccessLog,
			api.Recover,
			api.Timeout(cfg.RequestTimeout),
			api.NewRequestSignatureMiddleware(cfg.ClientSecrets, cfg.RequestSignatureMaxAge),
		),
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
		// ReadHeaderTimeout guards against slow-loris; the body read is bounded
		// separately so it doesn't also cap idle keep-alive connections.
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      cfg.RequestTimeout + 3*time.Second,
		// IdleTimeout must be set explicitly: when zero it defaults to
		// ReadTimeout, which would slam idle keep-alive connections shut after a
		// few seconds. Node's fetch (undici) pools and reuses sockets, so an
		// aggressive server-side close races client reuse and surfaces as
		// intermittent "other side closed" errors. Keep it comfortably longer
		// than any client keep-alive so the client always recycles first.
		IdleTimeout: 120 * time.Second,
	}

	listenErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	var runErr error
	select {
	case sig := <-quit:
		slog.Info("shutting down", "signal", sig.String())
	case err := <-listenErr:
		runErr = fmt.Errorf("listen: %w", err)
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("requests still in flight at shutdown", "err", err)
	}

	stopped := make(chan struct{})
	go func() {
		loops.Wait()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-shutdownCtx.Done():
		slog.Warn("background loops still running at shutdown")
	}
	return runErr
}

func healthChecks(cfg config.Config, db *store.Store, f *fetcher.Fetcher, svc *weather.Service) []api.HealthCheck {
	checks := []api.HealthCheck{
		{
			Name:     "database",
			Critical: true,
			Probe: func(ctx context.Context) api.HealthResult {
				if err := db.Ping(ctx); err != nil {
					slog.Warn("readiness: database ping failed", "err", err)
					return api.HealthResult{Detail: "ping failed"}
				}
				return api.HealthResult{OK: true}
			},
		},
		{
			Name:     "observations",
			Critical: true,
			Probe: func(context.Context) api.HealthResult {
				return api.FreshnessResult(f.Status().NewestObservationAt, maxObservationAge, time.Now())
			},
		},
	}
	if cfg.GribFetchEnable {
		checks = append(checks, api.HealthCheck{
			Name: "grib",
			Probe: func(context.Context) api.HealthResult {
				return api.FreshnessResult(svc.GribWarmedAt(), 2*cfg.GribInterval, time.Now())
			},
		})
	}
	if cfg.RadarFetchEnable {
		checks = append(checks, api.HealthCheck{
			Name: "radar",
			Probe: func(context.Context) api.HealthResult {
				return api.FreshnessResult(svc.NewestRadarGridAt(), maxRadarFrameAge, time.Now())
			},
		})
	}
	return checks
}
