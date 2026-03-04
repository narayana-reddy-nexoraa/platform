package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/config"
	"github.com/narayana-platform/execution-engine/internal/domain"
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

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
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
	claimer := worker.NewClaimer(svc, repo, cfg.WorkerID, logger)
	reaper := worker.NewReaper(svc, logger)

	// Event channel (buffered for backpressure)
	eventChan := make(chan domain.OutboxEvent, 1000)

	// Publisher & Consumer
	publisher := worker.NewPublisher(repo, eventChan, logger)
	consumer := worker.NewConsumer(repo, eventChan, "default", logger)

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

	logger.Info().Str("worker_id", cfg.WorkerID).Msg("worker started")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down worker...")
	cancel() // stops the claim loop
	logger.Info().Msg("worker stopped")
}
