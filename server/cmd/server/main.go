package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"wby/internal/api"
	"wby/internal/config"
	"wby/internal/fetcher"
	"wby/internal/fmi"
	"wby/internal/grib"
	"wby/internal/store"
	"wby/internal/weather"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	fmiClient := fmi.NewClientWithWMS(cfg.FMIBaseURL, cfg.FMIAPIKey, cfg.FMITimeseriesURL, cfg.FMIWMSBaseURL, cfg.FMIDownloadURL)

	svc := weather.NewService(db, fmiClient, 10*time.Minute)
	svc.SetPrecipitationLayers(cfg.FMIPrecipObsLayer, cfg.FMIPrecipFcstLayer)
	svc.SetPrecipitationStyle(cfg.FMIPrecipStyle)
	svc.SetGribTemperatureSource(grib.New(cfg.GribsvcURL, cfg.GribFilename, cfg.GribTempParam, cfg.GribStep))
	svc.SetGribPrecipitationSource(grib.NewPrecipitation(cfg.GribsvcURL, cfg.GribFilename, cfg.GribPrecipParam, cfg.GribPrecipStep))
	var radarGrib *grib.Client
	if cfg.RadarFetchEnable {
		radarGrib = grib.NewRadar(cfg.GribsvcURL, cfg.RadarGridStep)
		svc.SetRadarPrecipitationSource(radarGrib)
	}

	f := fetcher.New(fmiClient, db)
	go f.RunObservationLoop(ctx, 10*time.Minute)
	if cfg.GribFetchEnable {
		go f.RunGribLoop(ctx, fetcher.GribJob{
			DataDir:  cfg.GRIBDataDir,
			Filename: cfg.GribFilename,
			Producer: cfg.GribProducer,
			Params:   cfg.GribParams,
			BBox:     cfg.GribBBox,
		}, cfg.GribInterval, func(ctx context.Context) {
			var wg sync.WaitGroup
			wg.Go(func() { svc.WarmTemperatureGrids(ctx) })
			wg.Go(func() { svc.WarmPrecipitationGrids(ctx) })
			wg.Wait()
		})
	}
	if cfg.RadarFetchEnable {
		go f.RunRadarLoop(ctx, fmi.NewRadarClient(cfg.RadarWMSURL, cfg.RadarLayer), fetcher.RadarJob{
			DataDir: cfg.GRIBDataDir,
			BBox:    cfg.RadarBBox,
			Width:   cfg.RadarWidth,
			Height:  cfg.RadarHeight,
			Span:    cfg.RadarFrameSpan,
			Nowcast: radarGrib,
		}, func(ctx context.Context) {
			svc.WarmRadarGrids(ctx)
			svc.WarmNowcastGrids(ctx)
		})
	}
	// Disabled: bursts ~200 FMI WFS requests every 30min and on every restart,
	// which gets the server's IP rate-limited and starves the observation fetcher.
	// Re-enable only with bounded concurrency, jitter, and FMI error backoff.
	// go svc.RunForecastGridPrewarmLoop(ctx, 30*time.Minute)

	mux := http.NewServeMux()
	handler := api.NewHandler(svc)
	handler.RegisterRoutes(mux)
	signedMux := api.NewRequestSignatureMiddleware(cfg.ClientSecrets, cfg.RequestSignatureMaxAge)(mux)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: signedMux,
		// ReadHeaderTimeout guards against slow-loris; the body read is bounded
		// separately so it doesn't also cap idle keep-alive connections.
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		// IdleTimeout must be set explicitly: when zero it defaults to
		// ReadTimeout, which would slam idle keep-alive connections shut after a
		// few seconds. Node's fetch (undici) pools and reuses sockets, so an
		// aggressive server-side close races client reuse and surfaces as
		// intermittent "other side closed" errors. Keep it comfortably longer
		// than any client keep-alive so the client always recycles first.
		IdleTimeout: 120 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
	slog.Info("server stopped")
}
