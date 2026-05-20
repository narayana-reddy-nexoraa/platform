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

func TestClaimLifecycle_HappyPath(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateExecutions(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	payload := json.RawMessage(`{"task":"process-order"}`)

	// Create execution
	req := domain.CreateExecutionRequest{Payload: payload}
	exec, isNew, err := svc.CreateExecution(ctx, tenantID, "claim-test-key", req)
	require.NoError(t, err)
	assert.True(t, isNew)
	assert.Equal(t, domain.StatusCreated, exec.Status)

	// Claim it
	claimed, err := svc.ClaimNextExecution(ctx, "worker-test-1")
	require.NoError(t, err)
	assert.Equal(t, exec.ExecutionID, claimed.ExecutionID)
	assert.Equal(t, domain.StatusClaimed, claimed.Status)
	assert.Equal(t, int32(1), claimed.AttemptCount)
	assert.NotNil(t, claimed.LockedBy)
	assert.Equal(t, "worker-test-1", *claimed.LockedBy)

	// Transition to RUNNING
	running, err := svc.UpdateStatus(ctx, claimed.ExecutionID, domain.StatusRunning, claimed.Version)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusRunning, running.Status)

	// Complete it
	completed, err := svc.CompleteExecution(ctx, running.ExecutionID, "worker-test-1", running.Version)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, completed.Status)
	assert.Nil(t, completed.LockedBy, "lock should be released")
	assert.Nil(t, completed.LockExpiresAt, "lock expiry should be cleared")

	// Verify final state
	final, err := svc.GetExecution(ctx, exec.ExecutionID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, final.Status)
	assert.Equal(t, int32(1), final.AttemptCount)
}

func TestClaimLifecycle_FailAndRetry(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateExecutions(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	payload := json.RawMessage(`{"task":"may-fail"}`)

	// Create
	req := domain.CreateExecutionRequest{Payload: payload}
	exec, _, err := svc.CreateExecution(ctx, tenantID, "fail-retry-key", req)
	require.NoError(t, err)

	// Claim
	claimed, err := svc.ClaimNextExecution(ctx, "worker-1")
	require.NoError(t, err)

	// Transition to RUNNING
	running, err := svc.UpdateStatus(ctx, claimed.ExecutionID, domain.StatusRunning, claimed.Version)
	require.NoError(t, err)

	// Fail it
	failed, err := svc.FailExecution(ctx, running.ExecutionID, "worker-1", "TIMEOUT", "processing timed out", running.Version)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, failed.Status)
	assert.Nil(t, failed.LockedBy, "lock should be released on failure")

	// Should be claimable again (attempt 2)
	reclaimed, err := svc.ClaimNextExecution(ctx, "worker-2")
	require.NoError(t, err)
	assert.Equal(t, exec.ExecutionID, reclaimed.ExecutionID)
	assert.Equal(t, int32(2), reclaimed.AttemptCount)
	assert.Equal(t, "worker-2", *reclaimed.LockedBy, "different worker should claim retry")
}

func TestClaimLifecycle_NothingAvailable(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateExecutions(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	// No executions in the database
	_, err := svc.ClaimNextExecution(ctx, "worker-lonely")
	assert.Error(t, err)

	var claimErr *domain.ErrClaimFailed
	assert.ErrorAs(t, err, &claimErr)
}
