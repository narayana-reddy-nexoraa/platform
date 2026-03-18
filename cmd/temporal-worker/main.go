package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/config"
	"github.com/narayana-platform/execution-engine/internal/handler"
	"github.com/narayana-platform/execution-engine/internal/metrics"
	temporalpkg "github.com/narayana-platform/execution-engine/internal/temporal"
	"github.com/narayana-platform/execution-engine/internal/temporal/activities"
)

func main() {
	// Logger
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("component", "temporal-worker").Logger()

	// Config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	// Database connection pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to parse database config")
	}
	poolCfg.MaxConns = int32(cfg.DBMaxConns)
	poolCfg.MinConns = int32(cfg.DBMinConns)
	poolCfg.MaxConnLifetime = cfg.DBMaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.DBMaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create connection pool")
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal().Err(err).Msg("failed to ping database")
	}
	logger.Info().Msg("database connection established")

	// Create Temporal client
	temporalClient, err := temporalpkg.NewClient(cfg.TemporalHost, cfg.TemporalNamespace, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create Temporal client")
	}
	defer temporalClient.Close()
	logger.Info().
		Str("host", cfg.TemporalHost).
		Str("namespace", cfg.TemporalNamespace).
		Msg("Temporal client connected")

	// Create activities with database access
	acts := activities.NewActivities(pool, logger)

	// Start Temporal workers for all industry task queues
	workers := temporalpkg.StartWorkers(temporalClient, acts, logger)

	// Start all workers
	for _, w := range workers {
		if err := w.Start(); err != nil {
			logger.Fatal().Err(err).Msg("failed to start Temporal worker")
		}
	}
	logger.Info().Int("worker_count", len(workers)).Msg("all Temporal workers started")

	// Expose Prometheus metrics on :9090
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metrics.Handler())
	metricsSrv := &http.Server{Addr: ":9090", Handler: metricsMux}
	go func() {
		logger.Info().Str("addr", ":9090").Msg("metrics server starting")
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("metrics server failed")
		}
	}()

	// Health check server
	healthHandler := handler.NewHealthHandler(pool)
	healthSrv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.HealthPort),
		Handler: healthHandler.NetHTTPHandler(),
	}
	go func() {
		logger.Info().Str("addr", ":"+cfg.HealthPort).Msg("health server starting")
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("health server failed")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down Temporal workers...")

	// Stop all Temporal workers gracefully
	for _, w := range workers {
		w.Stop()
	}

	// Shutdown HTTP servers
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	metricsSrv.Shutdown(shutdownCtx)
	healthSrv.Shutdown(shutdownCtx)

	logger.Info().Msg("Temporal worker stopped")
}
