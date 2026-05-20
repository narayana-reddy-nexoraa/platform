package worker

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/metrics"
	"github.com/narayana-platform/execution-engine/internal/service"
)

const (
	reaperInterval  = 10 * time.Second
	reaperBatchSize = int32(100)
)

// Reaper scans for executions with expired leases and resets them to CREATED.
type Reaper struct {
	service *service.ExecutionService
	logger  zerolog.Logger
}

// NewReaper creates a new lease reaper.
func NewReaper(svc *service.ExecutionService, logger zerolog.Logger) *Reaper {
	return &Reaper{
		service: svc,
		logger:  logger.With().Str("component", "reaper").Logger(),
	}
}

// Run starts the reaper loop. Blocks until context is canceled.
func (r *Reaper) Run(ctx context.Context) {
	r.logger.Info().Msg("reaper started")
	ticker := time.NewTicker(reaperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info().Msg("reaper stopped")
			return
		case <-ticker.C:
			r.scan(ctx)
		}
	}
}

func (r *Reaper) scan(ctx context.Context) {
	expired, err := r.service.FindExpiredLeases(ctx, reaperBatchSize)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to find expired leases")
		return
	}

	if len(expired) == 0 {
		return
	}

	r.logger.Info().Int("count", len(expired)).Msg("found expired leases")

	for _, exec := range expired {
		_, err := r.service.ReclaimExecution(ctx, exec.ExecutionID, exec.Version)
		if err != nil {
			r.logger.Debug().
				Err(err).
				Str("execution_id", exec.ExecutionID.String()).
				Msg("reclaim failed (likely already reclaimed)")
			continue
		}

		metrics.LeasesReclaimedTotal.Inc()

		// If the execution has exhausted all attempts, count it as timed out
		if exec.AttemptCount >= exec.MaxAttempts {
			metrics.ExecutionsTimedOutTotal.Inc()
		}

		r.logger.Info().
			Str("execution_id", exec.ExecutionID.String()).
			Str("previous_worker", stringPtrOrEmpty(exec.LockedBy)).
			Msg("reclaimed expired execution")
	}
}

func stringPtrOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
