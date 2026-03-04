package worker

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/clock"
	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/metrics"
	"github.com/narayana-platform/execution-engine/internal/repository"
	"github.com/narayana-platform/execution-engine/internal/service"
)

const pollInterval = 2 * time.Second
const processDelay = 500 * time.Millisecond // simulated work

// Claimer runs a background loop that claims and processes executions.
type Claimer struct {
	service  *service.ExecutionService
	repo     repository.ExecutionRepository
	workerID string
	logger   zerolog.Logger
	wg       *sync.WaitGroup
	clock    clock.Clock
}

// NewClaimer creates a new claim loop worker.
func NewClaimer(svc *service.ExecutionService, repo repository.ExecutionRepository, workerID string, logger zerolog.Logger, wg *sync.WaitGroup, clk clock.Clock) *Claimer {
	return &Claimer{
		service:  svc,
		repo:     repo,
		workerID: workerID,
		logger:   logger.With().Str("worker_id", workerID).Logger(),
		wg:       wg,
		clock:    clk,
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
	claimQueryStart := cl.clock.Now()
	exec, err := cl.service.ClaimNextExecution(parentCtx, cl.workerID)
	metrics.ClaimQueryDurationSeconds.Observe(cl.clock.Now().Sub(claimQueryStart).Seconds())

	if err != nil {
		if _, ok := err.(*domain.ErrClaimFailed); ok {
			return
		}
		cl.logger.Error().Err(err).Msg("error claiming execution")
		return
	}

	cl.wg.Add(1)
	defer cl.wg.Done()

	metrics.ExecutionsClaimedTotal.Inc()
	metrics.QueueWaitSeconds.Observe(cl.clock.Now().Sub(exec.CreatedAt).Seconds())

	cl.logger.Info().
		Str("execution_id", exec.ExecutionID.String()).
		Int32("attempt", exec.AttemptCount).
		Msg("claimed execution")

	claimStart := cl.clock.Now()

	// Transition to RUNNING
	exec, err = cl.service.UpdateStatus(parentCtx, exec.ExecutionID, domain.StatusRunning, exec.Version)
	if err != nil {
		cl.logger.Error().Err(err).Str("execution_id", exec.ExecutionID.String()).Msg("failed to transition to RUNNING")
		return
	}

	// Write processing_log STARTED entry
	if logErr := cl.repo.InsertProcessingLog(parentCtx, exec.ExecutionID, cl.workerID, "STARTED", exec.AttemptCount); logErr != nil {
		cl.logger.Warn().Err(logErr).Str("execution_id", exec.ExecutionID.String()).Msg("failed to write STARTED processing log")
	}

	// Create cancellable context for this execution
	execCtx, cancelExec := context.WithCancel(parentCtx)

	// Start heartbeat goroutine
	hb := NewHeartbeat(cl.service, exec.ExecutionID, cl.workerID, cl.logger, cl.clock)
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
		metrics.ExecutionDurationSeconds.Observe(cl.clock.Now().Sub(claimStart).Seconds())
		cl.handleFailure(parentCtx, exec, processErr)
		return
	}

	// Mark as SUCCEEDED
	_, err = cl.service.CompleteExecution(parentCtx, exec.ExecutionID, cl.workerID, exec.Version)
	if err != nil {
		cl.logger.Error().Err(err).Str("execution_id", exec.ExecutionID.String()).Msg("failed to mark as SUCCEEDED")
		return
	}

	metrics.ExecutionDurationSeconds.Observe(cl.clock.Now().Sub(claimStart).Seconds())
	metrics.ExecutionsSucceededTotal.Inc()

	// Write processing_log COMPLETED entry
	if logErr := cl.repo.InsertProcessingLog(parentCtx, exec.ExecutionID, cl.workerID, "COMPLETED", exec.AttemptCount); logErr != nil {
		cl.logger.Warn().Err(logErr).Str("execution_id", exec.ExecutionID.String()).Msg("failed to write COMPLETED processing log")
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
		_, err := cl.service.RetryExecution(ctx, exec.ExecutionID, cl.workerID, errorCode, errorMessage, exec.AttemptCount, exec.Version)
		if err != nil {
			cl.logger.Error().Err(err).Str("execution_id", exec.ExecutionID.String()).Msg("failed to schedule retry")
		} else {
			metrics.ExecutionsRetriedTotal.Inc()
		}
		return
	}

	_, err := cl.service.FailExecution(ctx, exec.ExecutionID, cl.workerID, errorCode, errorMessage, exec.Version)
	if err != nil {
		cl.logger.Error().Err(err).Str("execution_id", exec.ExecutionID.String()).Msg("failed to mark as FAILED")
	} else {
		metrics.ExecutionsFailedTotal.Inc()
	}

	// Write processing_log ABORTED entry for permanent failures
	if logErr := cl.repo.InsertProcessingLog(ctx, exec.ExecutionID, cl.workerID, "ABORTED", exec.AttemptCount); logErr != nil {
		cl.logger.Warn().Err(logErr).Str("execution_id", exec.ExecutionID.String()).Msg("failed to write ABORTED processing log")
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
