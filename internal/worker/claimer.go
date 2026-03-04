package worker

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/service"
)

const pollInterval = 2 * time.Second
const processDelay = 500 * time.Millisecond // simulated work

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
func (cl *Claimer) poll(parentCtx context.Context) {
	exec, err := cl.service.ClaimNextExecution(parentCtx, cl.workerID)
	if err != nil {
		if _, ok := err.(*domain.ErrClaimFailed); ok {
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
	exec, err = cl.service.UpdateStatus(parentCtx, exec.ExecutionID, domain.StatusRunning, exec.Version)
	if err != nil {
		cl.logger.Error().Err(err).Str("execution_id", exec.ExecutionID.String()).Msg("failed to transition to RUNNING")
		return
	}

	// Create cancellable context for this execution
	execCtx, cancelExec := context.WithCancel(parentCtx)

	// Start heartbeat goroutine
	hb := NewHeartbeat(cl.service, exec.ExecutionID, cl.workerID, cl.logger)
	go hb.Run(execCtx, cancelExec)

	// Process
	processErr := cl.process(execCtx, exec)

	// Stop heartbeat
	cancelExec()

	// If heartbeat detected lost lease, don't try to update status
	if hb.WasStopped() {
		cl.logger.Warn().
			Str("execution_id", exec.ExecutionID.String()).
			Msg("lost lease during processing, skipping status update")
		return
	}

	if processErr != nil {
		cl.handleFailure(parentCtx, exec, processErr)
		return
	}

	// Mark as SUCCEEDED
	_, err = cl.service.CompleteExecution(parentCtx, exec.ExecutionID, exec.Version)
	if err != nil {
		cl.logger.Error().Err(err).Str("execution_id", exec.ExecutionID.String()).Msg("failed to mark as SUCCEEDED")
		return
	}

	cl.logger.Info().
		Str("execution_id", exec.ExecutionID.String()).
		Msg("execution completed successfully")
}

// handleFailure decides whether to retry or permanently fail an execution.
func (cl *Claimer) handleFailure(ctx context.Context, exec *domain.Execution, processErr error) {
	errorCode := "PROCESSING_ERROR"
	errorMessage := processErr.Error()

	if ee, ok := processErr.(*ExecutionError); ok {
		errorCode = ee.Code
		errorMessage = ee.Message
	}

	cl.logger.Warn().
		Str("execution_id", exec.ExecutionID.String()).
		Str("error_code", errorCode).
		Int32("attempt", exec.AttemptCount).
		Int32("max_attempts", exec.MaxAttempts).
		Msg("execution failed")

	if domain.IsRetryableError(errorCode) && exec.AttemptCount < exec.MaxAttempts {
		_, err := cl.service.RetryExecution(ctx, exec.ExecutionID, errorCode, errorMessage, exec.AttemptCount, exec.Version)
		if err != nil {
			cl.logger.Error().Err(err).Str("execution_id", exec.ExecutionID.String()).Msg("failed to schedule retry")
		}
		return
	}

	_, err := cl.service.FailExecution(ctx, exec.ExecutionID, errorCode, errorMessage, exec.Version)
	if err != nil {
		cl.logger.Error().Err(err).Str("execution_id", exec.ExecutionID.String()).Msg("failed to mark as FAILED")
	}
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

// ExecutionError is a typed error that carries an error code for retry classification.
type ExecutionError struct {
	Code    string
	Message string
}

func (e *ExecutionError) Error() string {
	return e.Code + ": " + e.Message
}
