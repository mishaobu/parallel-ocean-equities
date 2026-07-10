package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/analysis"
	"github.com/mishaobu/parallel-ocean-equities/internal/server"
	"github.com/mishaobu/parallel-ocean-equities/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	state, err := store.Open(env("DATA_FILE", "/data/state.json"), env("SEED_FILE", "/app/data/seed.json"), envInt("MAX_TICKERS", 30))
	if err != nil {
		logger.Error("open state", "error", err)
		os.Exit(1)
	}
	httpClient := &http.Client{Timeout: 2 * time.Minute}
	polygonAPIKey := os.Getenv("POLYGON_API_KEY")
	sec := analysis.NewSECClient(env("SEC_USER_AGENT", "parallel-ocean-equities parallel-ocean.xyz/equities"), polygonAPIKey, httpClient)
	market := analysis.NewCompositeMarket(
		analysis.NewYahooMarket(httpClient),
		analysis.NewThetaMarket(os.Getenv("THETA_BASE_URL"), httpClient),
		analysis.NewPolygonMarket(polygonAPIKey, httpClient),
	)
	pipeline := &analysis.Pipeline{SEC: sec, Market: market}
	service := analysis.NewService(state, pipeline).WithMacro(analysis.NewFREDClient(env("FRED_USER_AGENT", "parallel-ocean-equities/1.0 (https://parallel-ocean.xyz/equities)"), httpClient))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	service.Start(ctx, envInt("REFRESH_WORKERS", 2))
	if envBool("STARTUP_REFRESH", false) {
		go func() {
			select {
			case <-ctx.Done():
			case <-time.After(20 * time.Second):
				service.RefreshAll()
			}
		}()
	}
	if interval, err := time.ParseDuration(env("REFRESH_INTERVAL", "24h")); err == nil && interval > 0 {
		go scheduleRefresh(ctx, service, interval)
	}

	handler := server.New(service, server.Config{
		BasePath:     env("BASE_PATH", "/equities"),
		StaticDir:    env("STATIC_DIR", "/app/web"),
		RefreshToken: os.Getenv("REFRESH_TOKEN"),
		Logger:       logger,
	})
	httpServer := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           handler.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      3 * time.Minute,
		IdleTimeout:       75 * time.Second,
	}
	go func() {
		logger.Info("equities service started", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server", "error", err)
			cancel()
		}
	}()
	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func scheduleRefresh(ctx context.Context, service *analysis.Service, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.RefreshAll()
		}
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
