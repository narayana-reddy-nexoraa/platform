package worker

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/metrics"
	"github.com/narayana-platform/execution-engine/internal/repository"
)

const gaugeCollectorInterval = 15 * time.Second

// GaugeCollector periodically queries the database for aggregate counts
// and updates Prometheus gauges for active, pending, and unsent metrics.
type GaugeCollector struct {
	repo   repository.ExecutionRepository
	logger zerolog.Logger
}

// NewGaugeCollector creates a new gauge collector.
func NewGaugeCollector(repo repository.ExecutionRepository, logger zerolog.Logger) *GaugeCollector {
	return &GaugeCollector{
		repo:   repo,
		logger: logger.With().Str("component", "gauge_collector").Logger(),
	}
}

// Run starts the periodic gauge collection loop. Blocks until context is canceled.
func (g *GaugeCollector) Run(ctx context.Context) {
	g.logger.Info().Msg("gauge collector started")
	ticker := time.NewTicker(gaugeCollectorInterval)
	defer ticker.Stop()

	// Collect once immediately on startup
	g.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			g.logger.Info().Msg("gauge collector stopped")
			return
		case <-ticker.C:
			g.collect(ctx)
		}
	}
}

func (g *GaugeCollector) collect(ctx context.Context) {
	active, err := g.repo.CountActiveExecutions(ctx)
	if err != nil {
		g.logger.Error().Err(err).Msg("failed to count active executions")
	} else {
		metrics.ExecutionsActiveCount.Set(float64(active))
	}

	pending, err := g.repo.CountPendingExecutions(ctx)
	if err != nil {
		g.logger.Error().Err(err).Msg("failed to count pending executions")
	} else {
		metrics.ExecutionsPendingCount.Set(float64(pending))
	}

	unsent, err := g.repo.CountUnsentEvents(ctx)
	if err != nil {
		g.logger.Error().Err(err).Msg("failed to count unsent events")
	} else {
		metrics.OutboxUnsentCount.Set(float64(unsent))
	}
}
