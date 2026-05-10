package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wby/internal/api"
	"wby/internal/config"
	"wby/internal/fetcher"
	"wby/internal/fmi"
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

	fmiClient := fmi.NewClientWithWMS(cfg.FMIBaseURL, cfg.FMIAPIKey, cfg.FMITimeseriesURL, cfg.FMIWMSBaseURL)

	svc := weather.NewService(db, fmiClient, 10*time.Minute)
	svc.SetPrecipitationLayers(cfg.FMIPrecipObsLayer, cfg.FMIPrecipFcstLayer)
	svc.SetPrecipitationStyle(cfg.FMIPrecipStyle)

	f := fetcher.New(fmiClient, db)
	go f.RunObservationLoop(ctx, 10*time.Minute)
	// Disabled: bursts ~200 FMI WFS requests every 30min and on every restart,
	// which gets the server's IP rate-limited and starves the observation fetcher.
	// Re-enable only with bounded concurrency, jitter, and FMI error backoff.
	// go svc.RunForecastGridPrewarmLoop(ctx, 30*time.Minute)

	mux := http.NewServeMux()
	handler := api.NewHandler(svc)
	handler.RegisterRoutes(mux)
	signedMux := api.NewRequestSignatureMiddleware(cfg.ClientSecrets, cfg.RequestSignatureMaxAge)(mux)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      signedMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
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
