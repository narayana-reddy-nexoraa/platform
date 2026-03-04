package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/narayana-platform/execution-engine/internal/domain"
)

// mockRepo implements repository.ExecutionRepository for unit testing.
type mockRepo struct {
	createIdempotentFn func(ctx context.Context, tenantID uuid.UUID, idempotencyKey string, maxAttempts int32, payload json.RawMessage, payloadHash string) (*domain.Execution, bool, error)
	getByIDFn          func(ctx context.Context, executionID, tenantID uuid.UUID) (*domain.Execution, error)
	listFn             func(ctx context.Context, tenantID uuid.UUID, status *domain.ExecutionStatus, limit, offset int32) ([]domain.Execution, int64, error)
	findClaimableFn    func(ctx context.Context, limit int32) ([]domain.Execution, error)
	claimFn            func(ctx context.Context, executionID uuid.UUID, workerID string, leaseDuration int32, version int32) (*domain.Execution, error)
	updateStatusFn     func(ctx context.Context, executionID uuid.UUID, status domain.ExecutionStatus, version int32) (*domain.Execution, error)
	completeFn         func(ctx context.Context, executionID uuid.UUID, version int32) (*domain.Execution, error)
	failFn             func(ctx context.Context, executionID uuid.UUID, errorCode, errorMessage string, version int32) (*domain.Execution, error)
}

func (m *mockRepo) CreateIdempotent(ctx context.Context, tenantID uuid.UUID, idempotencyKey string, maxAttempts int32, payload json.RawMessage, payloadHash string) (*domain.Execution, bool, error) {
	return m.createIdempotentFn(ctx, tenantID, idempotencyKey, maxAttempts, payload, payloadHash)
}
func (m *mockRepo) GetByID(ctx context.Context, executionID, tenantID uuid.UUID) (*domain.Execution, error) {
	return m.getByIDFn(ctx, executionID, tenantID)
}
func (m *mockRepo) List(ctx context.Context, tenantID uuid.UUID, status *domain.ExecutionStatus, limit, offset int32) ([]domain.Execution, int64, error) {
	return m.listFn(ctx, tenantID, status, limit, offset)
}
func (m *mockRepo) FindClaimable(ctx context.Context, limit int32) ([]domain.Execution, error) {
	return m.findClaimableFn(ctx, limit)
}
func (m *mockRepo) Claim(ctx context.Context, executionID uuid.UUID, workerID string, leaseDuration int32, version int32) (*domain.Execution, error) {
	return m.claimFn(ctx, executionID, workerID, leaseDuration, version)
}
func (m *mockRepo) UpdateStatus(ctx context.Context, executionID uuid.UUID, status domain.ExecutionStatus, version int32) (*domain.Execution, error) {
	return m.updateStatusFn(ctx, executionID, status, version)
}
func (m *mockRepo) Complete(ctx context.Context, executionID uuid.UUID, version int32) (*domain.Execution, error) {
	return m.completeFn(ctx, executionID, version)
}
func (m *mockRepo) Fail(ctx context.Context, executionID uuid.UUID, errorCode, errorMessage string, version int32) (*domain.Execution, error) {
	return m.failFn(ctx, executionID, errorCode, errorMessage, version)
}

func newTestService(repo *mockRepo) *ExecutionService {
	logger := zerolog.Nop()
	return NewExecutionService(repo, 30, 10, logger)
}

func TestCreateExecution_DefaultMaxAttempts(t *testing.T) {
	tenantID := uuid.New()
	var capturedMaxAttempts int32

	repo := &mockRepo{
		createIdempotentFn: func(ctx context.Context, tid uuid.UUID, key string, maxAttempts int32, payload json.RawMessage, hash string) (*domain.Execution, bool, error) {
			capturedMaxAttempts = maxAttempts
			return &domain.Execution{
				ExecutionID: uuid.New(),
				TenantID:    tid,
				Status:      domain.StatusCreated,
				MaxAttempts: maxAttempts,
				Version:     1,
			}, true, nil
		},
	}

	svc := newTestService(repo)
	req := domain.CreateExecutionRequest{
		Payload: json.RawMessage(`{"test": true}`),
		// MaxAttempts not set — should default to 3
	}

	_, _, err := svc.CreateExecution(context.Background(), tenantID, "key-1", req)
	require.NoError(t, err)
	assert.Equal(t, int32(3), capturedMaxAttempts, "should default to 3 max attempts")
}

func TestCreateExecution_CustomMaxAttempts(t *testing.T) {
	tenantID := uuid.New()
	var capturedMaxAttempts int32

	repo := &mockRepo{
		createIdempotentFn: func(ctx context.Context, tid uuid.UUID, key string, maxAttempts int32, payload json.RawMessage, hash string) (*domain.Execution, bool, error) {
			capturedMaxAttempts = maxAttempts
			return &domain.Execution{
				ExecutionID: uuid.New(),
				TenantID:    tid,
				Status:      domain.StatusCreated,
				MaxAttempts: maxAttempts,
				Version:     1,
			}, true, nil
		},
	}

	svc := newTestService(repo)
	maxAttempts := int32(5)
	req := domain.CreateExecutionRequest{
		Payload:     json.RawMessage(`{"test": true}`),
		MaxAttempts: &maxAttempts,
	}

	_, _, err := svc.CreateExecution(context.Background(), tenantID, "key-1", req)
	require.NoError(t, err)
	assert.Equal(t, int32(5), capturedMaxAttempts)
}

func TestCreateExecution_InvalidJSON(t *testing.T) {
	svc := newTestService(&mockRepo{})
	req := domain.CreateExecutionRequest{
		Payload: json.RawMessage(`{not valid`),
	}

	_, _, err := svc.CreateExecution(context.Background(), uuid.New(), "key-1", req)
	assert.Error(t, err)

	var validationErr *domain.ErrValidation
	assert.ErrorAs(t, err, &validationErr)
}

func TestListExecutions_BoundsEnforcement(t *testing.T) {
	tenantID := uuid.New()
	var capturedLimit, capturedOffset int32

	repo := &mockRepo{
		listFn: func(ctx context.Context, tid uuid.UUID, status *domain.ExecutionStatus, limit, offset int32) ([]domain.Execution, int64, error) {
			capturedLimit = limit
			capturedOffset = offset
			return []domain.Execution{}, 0, nil
		},
	}

	svc := newTestService(repo)

	// Limit too high → capped at 100
	_, err := svc.ListExecutions(context.Background(), tenantID, nil, 500, 0)
	require.NoError(t, err)
	assert.Equal(t, int32(100), capturedLimit)

	// Negative offset → clamped to 0
	_, err = svc.ListExecutions(context.Background(), tenantID, nil, 20, -5)
	require.NoError(t, err)
	assert.Equal(t, int32(0), capturedOffset)

	// Zero limit → defaults to 20
	_, err = svc.ListExecutions(context.Background(), tenantID, nil, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, int32(20), capturedLimit)
}

func TestClaimNextExecution_BatchStrategy(t *testing.T) {
	execID := uuid.New()
	claimAttempts := 0

	repo := &mockRepo{
		findClaimableFn: func(ctx context.Context, limit int32) ([]domain.Execution, error) {
			return []domain.Execution{
				{ExecutionID: uuid.New(), Version: 1},  // will fail
				{ExecutionID: execID, Version: 1},       // will succeed
				{ExecutionID: uuid.New(), Version: 1},  // should never be tried
			}, nil
		},
		claimFn: func(ctx context.Context, eid uuid.UUID, workerID string, lease int32, version int32) (*domain.Execution, error) {
			claimAttempts++
			if eid == execID {
				return &domain.Execution{ExecutionID: execID, Status: domain.StatusClaimed}, nil
			}
			return nil, &domain.ErrOptimisticLock{ExecutionID: eid.String()}
		},
	}

	svc := newTestService(repo)
	exec, err := svc.ClaimNextExecution(context.Background(), "worker-1")
	require.NoError(t, err)
	assert.Equal(t, execID, exec.ExecutionID)
	assert.Equal(t, 2, claimAttempts, "should have tried 2 candidates (first failed, second succeeded)")
}

func TestClaimNextExecution_NoCandidates(t *testing.T) {
	repo := &mockRepo{
		findClaimableFn: func(ctx context.Context, limit int32) ([]domain.Execution, error) {
			return []domain.Execution{}, nil
		},
	}

	svc := newTestService(repo)
	_, err := svc.ClaimNextExecution(context.Background(), "worker-1")
	assert.Error(t, err)

	var claimErr *domain.ErrClaimFailed
	assert.ErrorAs(t, err, &claimErr)
}
