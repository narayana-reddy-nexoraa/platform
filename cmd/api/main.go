package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/config"
	"github.com/narayana-platform/execution-engine/internal/handler"
	"github.com/narayana-platform/execution-engine/internal/metrics"
	"github.com/narayana-platform/execution-engine/internal/middleware"
	"github.com/narayana-platform/execution-engine/internal/repository"
	"github.com/narayana-platform/execution-engine/internal/service"
)

func main() {
	// Logger
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("component", "api").Logger()

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
	logger.Info().Msg("database connection established")

	// Wire dependencies
	repo := repository.NewPostgresExecutionRepository(pool)
	svc := service.NewExecutionService(repo, int32(cfg.LeaseDurationSeconds), int32(cfg.ClaimBatchSize), logger)
	h := handler.NewExecutionHandler(svc)

	// Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Global middleware (applied to all routes)
	router.Use(middleware.ErrorHandler(logger))
	router.Use(middleware.CorrelationID())
	router.Use(middleware.RequestLogger(logger))

	// Health endpoints (no tenant required)
	healthHandler := handler.NewHealthHandler(pool)
	healthHandler.RegisterRoutes(router)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(metrics.Handler()))

	// API v1 routes (tenant required)
	v1 := router.Group("/api/v1")
	v1.Use(middleware.TenantExtractor())
	{
		v1.POST("/executions", h.CreateExecution)
		v1.GET("/executions/:id", h.GetExecution)
		v1.GET("/executions", h.ListExecutions)
	}

	// HTTP server with graceful shutdown
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.ServerPort),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		logger.Info().Str("port", cfg.ServerPort).Msg("API server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server failed")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down API server...")

	// Give in-flight requests 10 seconds to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal().Err(err).Msg("server forced to shutdown")
	}

	logger.Info().Msg("API server stopped")
}
