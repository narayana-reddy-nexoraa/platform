package worker

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/service"
)

const pollInterval = 2 * time.Second
const processDelay = 500 * time.Millisecond // simulated work for Week 1

// Claimer runs a background loop that claims and processes executions.
type Claimer struct {
	service  *service.ExecutionService
	workerID string
	logger   zerolog.Logger
}

// NewClaimer creates a new claim loop worker.
func NewClaimer(svc *service.ExecutionService, workerID string, logger zerolog.Logger) *Claimer {
	return &Claimer{
		service:  svc,
		workerID: workerID,
		logger:   logger.With().Str("worker_id", workerID).Logger(),
	}
}

// Run starts the polling loop. Blocks until context is canceled.
func (cl *Claimer) Run(ctx context.Context) {
	cl.logger.Info().Msg("claim loop started")
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			cl.logger.Info().Msg("claim loop stopped")
			return
		case <-ticker.C:
			cl.poll(ctx)
		}
	}
}

// poll attempts to claim one execution and process it.
func (cl *Claimer) poll(ctx context.Context) {
	// Try to claim
	exec, err := cl.service.ClaimNextExecution(ctx, cl.workerID)
	if err != nil {
		if _, ok := err.(*domain.ErrClaimFailed); ok {
			// No work available — this is normal, not an error
			return
		}
		cl.logger.Error().Err(err).Msg("error claiming execution")
		return
	}

	cl.logger.Info().
		Str("execution_id", exec.ExecutionID.String()).
		Int32("attempt", exec.AttemptCount).
		Msg("claimed execution")

	// Transition to RUNNING
	exec, err = cl.service.UpdateStatus(ctx, exec.ExecutionID, domain.StatusRunning, exec.Version)
	if err != nil {
		cl.logger.Error().Err(err).Str("execution_id", exec.ExecutionID.String()).Msg("failed to transition to RUNNING")
		return
	}

	// Process (simulated work for Week 1)
	err = cl.process(ctx, exec)
	if err != nil {
		cl.logger.Error().Err(err).Str("execution_id", exec.ExecutionID.String()).Msg("processing failed")
		_, failErr := cl.service.FailExecution(ctx, exec.ExecutionID, "PROCESSING_ERROR", err.Error(), exec.Version)
		if failErr != nil {
			cl.logger.Error().Err(failErr).Str("execution_id", exec.ExecutionID.String()).Msg("failed to mark as FAILED")
		}
		return
	}

	// Mark as SUCCEEDED
	_, err = cl.service.CompleteExecution(ctx, exec.ExecutionID, exec.Version)
	if err != nil {
		cl.logger.Error().Err(err).Str("execution_id", exec.ExecutionID.String()).Msg("failed to mark as SUCCEEDED")
		return
	}

	cl.logger.Info().
		Str("execution_id", exec.ExecutionID.String()).
		Msg("execution completed successfully")
}

// process simulates work. In future weeks, this will execute real job logic.
func (cl *Claimer) process(ctx context.Context, exec *domain.Execution) error {
	cl.logger.Info().
		Str("execution_id", exec.ExecutionID.String()).
		Msg("processing execution (simulated)")

	select {
	case <-time.After(processDelay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
