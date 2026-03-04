package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
	"github.com/narayana-platform/execution-engine/internal/service"
)

func TestRetry_RetryableError_SchedulesRetry(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	maxAttempts := int32(3)
	payload := json.RawMessage(`{"task":"retryable"}`)

	req := domain.CreateExecutionRequest{Payload: payload, MaxAttempts: &maxAttempts}
	exec, _, err := svc.CreateExecution(ctx, tenantID, "retry-test-1", req)
	require.NoError(t, err)

	claimed, err := svc.ClaimNextExecution(ctx, "worker-1")
	require.NoError(t, err)
	running, err := svc.UpdateStatus(ctx, claimed.ExecutionID, domain.StatusRunning, claimed.Version)
	require.NoError(t, err)

	retried, err := svc.RetryExecution(ctx, running.ExecutionID, "DOWNSTREAM_TIMEOUT", "timed out", running.AttemptCount, running.Version)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCreated, retried.Status)
	assert.NotNil(t, retried.RetryAfter)
	assert.NotNil(t, retried.ErrorCode)
	assert.Equal(t, "DOWNSTREAM_TIMEOUT", *retried.ErrorCode)

	fetched, err := svc.GetExecution(ctx, exec.ExecutionID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCreated, fetched.Status)
}

func TestRetry_NonRetryableError_TerminalFail(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	payload := json.RawMessage(`{"task":"non-retryable"}`)

	req := domain.CreateExecutionRequest{Payload: payload}
	_, _, err := svc.CreateExecution(ctx, tenantID, "retry-test-2", req)
	require.NoError(t, err)

	claimed, err := svc.ClaimNextExecution(ctx, "worker-1")
	require.NoError(t, err)
	running, err := svc.UpdateStatus(ctx, claimed.ExecutionID, domain.StatusRunning, claimed.Version)
	require.NoError(t, err)

	assert.False(t, domain.IsRetryableError("VALIDATION_FAILED"))
	failed, err := svc.FailExecution(ctx, running.ExecutionID, "VALIDATION_FAILED", "bad input", running.Version)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, failed.Status)
}

func TestRetry_MaxAttemptsExhausted_TerminalFail(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	maxAttempts := int32(2)
	payload := json.RawMessage(`{"task":"max-attempts"}`)

	req := domain.CreateExecutionRequest{Payload: payload, MaxAttempts: &maxAttempts}
	exec, _, err := svc.CreateExecution(ctx, tenantID, "retry-test-3", req)
	require.NoError(t, err)

	// Attempt 1
	claimed, err := svc.ClaimNextExecution(ctx, "worker-1")
	require.NoError(t, err)
	running, err := svc.UpdateStatus(ctx, claimed.ExecutionID, domain.StatusRunning, claimed.Version)
	require.NoError(t, err)
	_, err = svc.RetryExecution(ctx, running.ExecutionID, "DOWNSTREAM_TIMEOUT", "timed out", running.AttemptCount, running.Version)
	require.NoError(t, err)

	// Clear retry_after so it's immediately claimable
	_, err = testPool.Exec(ctx, "UPDATE executions SET retry_after = NULL WHERE execution_id = $1", exec.ExecutionID)
	require.NoError(t, err)

	// Attempt 2: at max_attempts
	claimed2, err := svc.ClaimNextExecution(ctx, "worker-2")
	require.NoError(t, err)
	assert.Equal(t, int32(2), claimed2.AttemptCount)
	running2, err := svc.UpdateStatus(ctx, claimed2.ExecutionID, domain.StatusRunning, claimed2.Version)
	require.NoError(t, err)

	assert.Equal(t, running2.AttemptCount, running2.MaxAttempts)
	failed, err := svc.FailExecution(ctx, running2.ExecutionID, "DOWNSTREAM_TIMEOUT", "timed out again", running2.Version)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, failed.Status)

	// Should NOT be claimable
	_, err = testPool.Exec(ctx, "UPDATE executions SET retry_after = NULL WHERE execution_id = $1", exec.ExecutionID)
	require.NoError(t, err)
	_, claimErr := svc.ClaimNextExecution(ctx, "worker-3")
	assert.Error(t, claimErr)
}

func TestRetry_RetryAfter_RespectsDelay(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	payload := json.RawMessage(`{"task":"delay-test"}`)

	req := domain.CreateExecutionRequest{Payload: payload}
	_, _, err := svc.CreateExecution(ctx, tenantID, "retry-delay", req)
	require.NoError(t, err)

	claimed, err := svc.ClaimNextExecution(ctx, "worker-1")
	require.NoError(t, err)
	running, err := svc.UpdateStatus(ctx, claimed.ExecutionID, domain.StatusRunning, claimed.Version)
	require.NoError(t, err)

	_, err = svc.RetryExecution(ctx, running.ExecutionID, "DOWNSTREAM_TIMEOUT", "timed out", running.AttemptCount, running.Version)
	require.NoError(t, err)

	// Should NOT be claimable immediately (retry_after is in the future)
	_, err = svc.ClaimNextExecution(ctx, "worker-2")
	assert.Error(t, err)
}
