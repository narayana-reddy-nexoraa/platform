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

func TestStateMachine_ValidTransitions(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	payload := json.RawMessage(`{"task":"state-machine-test"}`)

	req := domain.CreateExecutionRequest{Payload: payload}
	exec, _, err := svc.CreateExecution(ctx, tenantID, "sm-happy-path", req)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCreated, exec.Status)

	claimed, err := svc.ClaimNextExecution(ctx, "worker-sm")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusClaimed, claimed.Status)

	running, err := svc.UpdateStatus(ctx, claimed.ExecutionID, domain.StatusRunning, claimed.Version)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusRunning, running.Status)

	completed, err := svc.CompleteExecution(ctx, running.ExecutionID, running.Version)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, completed.Status)
}

func TestStateMachine_FailAndRetryPath(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	payload := json.RawMessage(`{"task":"retry-path"}`)

	req := domain.CreateExecutionRequest{Payload: payload}
	exec, _, err := svc.CreateExecution(ctx, tenantID, "sm-retry", req)
	require.NoError(t, err)

	claimed, err := svc.ClaimNextExecution(ctx, "worker-1")
	require.NoError(t, err)
	running, err := svc.UpdateStatus(ctx, claimed.ExecutionID, domain.StatusRunning, claimed.Version)
	require.NoError(t, err)

	retried, err := svc.RetryExecution(ctx, running.ExecutionID, "DOWNSTREAM_TIMEOUT", "service timed out", running.AttemptCount, running.Version)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCreated, retried.Status)
	assert.Nil(t, retried.LockedBy)
	assert.NotNil(t, retried.RetryAfter)

	fetched, err := svc.GetExecution(ctx, exec.ExecutionID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCreated, fetched.Status)
	assert.NotNil(t, fetched.ErrorCode)
	assert.Equal(t, "DOWNSTREAM_TIMEOUT", *fetched.ErrorCode)
}

func TestStateMachine_AuditTrail(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	payload := json.RawMessage(`{"task":"audit-test"}`)

	req := domain.CreateExecutionRequest{Payload: payload}
	exec, _, err := svc.CreateExecution(ctx, tenantID, "sm-audit", req)
	require.NoError(t, err)

	claimed, err := svc.ClaimNextExecution(ctx, "worker-audit")
	require.NoError(t, err)
	running, err := svc.UpdateStatus(ctx, claimed.ExecutionID, domain.StatusRunning, claimed.Version)
	require.NoError(t, err)
	_, err = svc.RetryExecution(ctx, running.ExecutionID, "DOWNSTREAM_5XX", "502 bad gateway", running.AttemptCount, running.Version)
	require.NoError(t, err)

	var count int64
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM execution_transitions WHERE execution_id = $1", exec.ExecutionID).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(1))
}
