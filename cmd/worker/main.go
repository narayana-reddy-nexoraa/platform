package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/config"
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
	claimer := worker.NewClaimer(svc, cfg.WorkerID, logger)
	reaper := worker.NewReaper(svc, logger)

	// Start claim loop
	go claimer.Run(ctx)
	go reaper.Run(ctx)

	logger.Info().Str("worker_id", cfg.WorkerID).Msg("worker started")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down worker...")
	cancel() // stops the claim loop
	logger.Info().Msg("worker stopped")
}
