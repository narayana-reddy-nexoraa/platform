package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
)

const defaultMaxAttempts int32 = 3

// ExecutionService contains the business logic for managing executions.
type ExecutionService struct {
	repo           repository.ExecutionRepository
	leaseDuration  int32 // seconds
	claimBatchSize int32
	logger         zerolog.Logger
}

// NewExecutionService creates a new service instance.
func NewExecutionService(
	repo repository.ExecutionRepository,
	leaseDuration int32,
	claimBatchSize int32,
	logger zerolog.Logger,
) *ExecutionService {
	return &ExecutionService{
		repo:           repo,
		leaseDuration:  leaseDuration,
		claimBatchSize: claimBatchSize,
		logger:         logger,
	}
}

// CreateExecution validates input, computes payload hash, and creates an execution idempotently.
func (s *ExecutionService) CreateExecution(
	ctx context.Context,
	tenantID uuid.UUID,
	idempotencyKey string,
	req domain.CreateExecutionRequest,
) (*domain.Execution, bool, error) {
	// Validate payload is valid JSON
	if !json.Valid(req.Payload) {
		return nil, false, &domain.ErrValidation{Field: "payload", Message: "must be valid JSON"}
	}

	// Default max_attempts if not provided
	maxAttempts := defaultMaxAttempts
	if req.MaxAttempts != nil && *req.MaxAttempts > 0 {
		maxAttempts = *req.MaxAttempts
	}

	// Compute deterministic hash for idempotency conflict detection
	payloadHash, err := domain.ComputePayloadHash(req.Payload)
	if err != nil {
		return nil, false, &domain.ErrValidation{Field: "payload", Message: err.Error()}
	}

	return s.repo.CreateIdempotent(ctx, tenantID, idempotencyKey, maxAttempts, req.Payload, payloadHash)
}

// GetExecution retrieves an execution by ID, scoped to a tenant.
func (s *ExecutionService) GetExecution(ctx context.Context, executionID, tenantID uuid.UUID) (*domain.Execution, error) {
	return s.repo.GetByID(ctx, executionID, tenantID)
}

// ListExecutions returns a paginated list of executions for a tenant.
func (s *ExecutionService) ListExecutions(ctx context.Context, tenantID uuid.UUID, status *domain.ExecutionStatus, limit, offset int32) (*domain.PaginatedResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	executions, totalCount, err := s.repo.List(ctx, tenantID, status, limit, offset)
	if err != nil {
		return nil, err
	}

	data := make([]domain.ExecutionResponse, len(executions))
	for i := range executions {
		data[i] = executions[i].ToResponse()
	}

	return &domain.PaginatedResponse{
		Data:       data,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

// ClaimNextExecution fetches a batch of claimable candidates and tries to claim each one.
// Returns the first successfully claimed execution, or ErrClaimFailed if none available.
func (s *ExecutionService) ClaimNextExecution(ctx context.Context, workerID string) (*domain.Execution, error) {
	candidates, err := s.repo.FindClaimable(ctx, s.claimBatchSize)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, &domain.ErrClaimFailed{}
	}

	for _, candidate := range candidates {
		claimed, err := s.repo.Claim(ctx, candidate.ExecutionID, workerID, s.leaseDuration, candidate.Version)
		if err != nil {
			// Optimistic lock failure — another worker got it, try next
			s.logger.Debug().
				Str("execution_id", candidate.ExecutionID.String()).
				Str("worker_id", workerID).
				Msg("claim attempt failed, trying next candidate")
			continue
		}
		return claimed, nil
	}

	return nil, &domain.ErrClaimFailed{}
}

// UpdateStatus transitions an execution to a new status with optimistic locking.
func (s *ExecutionService) UpdateStatus(ctx context.Context, executionID uuid.UUID, status domain.ExecutionStatus, version int32) (*domain.Execution, error) {
	return s.repo.UpdateStatus(ctx, executionID, status, version)
}

// CompleteExecution marks an execution as SUCCEEDED and releases the lease.
func (s *ExecutionService) CompleteExecution(ctx context.Context, executionID uuid.UUID, version int32) (*domain.Execution, error) {
	return s.repo.Complete(ctx, executionID, version)
}

// FailExecution marks an execution as FAILED, records the error, and releases the lease.
func (s *ExecutionService) FailExecution(ctx context.Context, executionID uuid.UUID, errorCode, errorMessage string, version int32) (*domain.Execution, error) {
	return s.repo.Fail(ctx, executionID, errorCode, errorMessage, version)
}

// RetryExecution re-queues a failed execution with a calculated backoff delay.
func (s *ExecutionService) RetryExecution(ctx context.Context, executionID uuid.UUID, errorCode, errorMessage string, attemptCount int32, version int32) (*domain.Execution, error) {
	delay := domain.DefaultRetryPolicy.CalculateDelay(attemptCount)
	delayMs := delay.Milliseconds()

	exec, err := s.repo.Retry(ctx, executionID, errorCode, errorMessage, delayMs, version)
	if err != nil {
		return nil, err
	}

	_ = s.repo.InsertTransition(ctx, executionID, domain.StatusRunning, domain.StatusCreated, "retry-engine",
		fmt.Sprintf("Retry attempt %d, delay %dms, error: %s", attemptCount, delayMs, errorCode))

	s.logger.Info().
		Str("execution_id", executionID.String()).
		Int32("attempt", attemptCount).
		Int64("delay_ms", delayMs).
		Str("error_code", errorCode).
		Msg("execution scheduled for retry")

	return exec, nil
}

// SendHeartbeat extends the lease for an active execution.
func (s *ExecutionService) SendHeartbeat(ctx context.Context, executionID uuid.UUID, workerID string) (*domain.Execution, error) {
	return s.repo.SendHeartbeat(ctx, executionID, s.leaseDuration, workerID)
}

// FindExpiredLeases returns executions with expired locks.
func (s *ExecutionService) FindExpiredLeases(ctx context.Context, limit int32) ([]domain.Execution, error) {
	return s.repo.FindExpiredLeases(ctx, limit)
}

// ReclaimExecution resets an expired execution back to CREATED.
func (s *ExecutionService) ReclaimExecution(ctx context.Context, executionID uuid.UUID, version int32) (*domain.Execution, error) {
	exec, err := s.repo.Reclaim(ctx, executionID, version)
	if err != nil {
		return nil, err
	}

	_ = s.repo.InsertTransition(ctx, executionID, domain.StatusClaimed, domain.StatusCreated, "reaper", "Lease expired, reclaimed")

	return exec, nil
}
