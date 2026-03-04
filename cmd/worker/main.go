package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/clock"
	"github.com/narayana-platform/execution-engine/internal/config"
	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/handler"
	"github.com/narayana-platform/execution-engine/internal/metrics"
	"github.com/narayana-platform/execution-engine/internal/repository"
	"github.com/narayana-platform/execution-engine/internal/service"
	"github.com/narayana-platform/execution-engine/internal/worker"
)

func main() {
	// Logger
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("component", "worker").Logger()

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

	// Verify database connectivity
	if err := pool.Ping(ctx); err != nil {
		logger.Fatal().Err(err).Msg("failed to ping database")
	}
	logger.Info().Str("worker_id", cfg.WorkerID).Msg("database connection established")

	// Wire dependencies
	repo := repository.NewPostgresExecutionRepository(pool)
	svc := service.NewExecutionService(repo, int32(cfg.LeaseDurationSeconds), int32(cfg.ClaimBatchSize), logger)
	clk := clock.RealClock{}
	var wg sync.WaitGroup
	claimer := worker.NewClaimer(svc, repo, cfg.WorkerID, logger, &wg, clk, cfg.FailureRate)
	reaper := worker.NewReaper(svc, logger)

	// Event channel (buffered for backpressure)
	eventChan := make(chan domain.OutboxEvent, 1000)

	// Publisher & Consumer
	publisher := worker.NewPublisher(repo, eventChan, logger, clk)
	consumer := worker.NewConsumer(repo, eventChan, "default", logger, clk)

	// Gauge collector
	gc := worker.NewGaugeCollector(repo, logger)

	// Start background goroutines
	go claimer.Run(ctx)
	go reaper.Run(ctx)
	go publisher.Run(ctx)
	go consumer.Run(ctx)
	go gc.Run(ctx)

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

	// Health check server on configurable port (default :8081)
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

	logger.Info().Str("worker_id", cfg.WorkerID).Msg("worker started")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down worker, cancelling context...")
	cancel() // stops the claim loop (no new claims will be made)

	// Wait for in-flight executions to drain with configurable timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	logger.Info().Msg("draining in-flight executions")
	select {
	case <-done:
		logger.Info().Msg("all in-flight work drained successfully")
	case <-time.After(time.Duration(cfg.ShutdownTimeoutSeconds) * time.Second):
		logger.Warn().Msg("shutdown timeout exceeded, forcing exit")
	}

	logger.Info().Msg("worker stopped")
}
