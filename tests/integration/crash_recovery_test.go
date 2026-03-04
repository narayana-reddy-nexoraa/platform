package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
	"github.com/narayana-platform/execution-engine/internal/service"
)

func TestCrashRecovery_ReaperReclaims(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 1, 10, logger) // 1s lease

	tenantID := uuid.New()
	payload := json.RawMessage(`{"task":"crash-test"}`)

	req := domain.CreateExecutionRequest{Payload: payload}
	exec, _, err := svc.CreateExecution(ctx, tenantID, "crash-key", req)
	require.NoError(t, err)

	claimed, err := svc.ClaimNextExecution(ctx, "worker-A")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusClaimed, claimed.Status)

	// Simulate crash: no heartbeat, wait for lease to expire
	time.Sleep(2 * time.Second)

	expired, err := svc.FindExpiredLeases(ctx, 100)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, exec.ExecutionID, expired[0].ExecutionID)

	reclaimed, err := svc.ReclaimExecution(ctx, expired[0].ExecutionID, expired[0].Version)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCreated, reclaimed.Status)
	assert.Nil(t, reclaimed.LockedBy)
	assert.Nil(t, reclaimed.LockExpiresAt)

	claimedByB, err := svc.ClaimNextExecution(ctx, "worker-B")
	require.NoError(t, err)
	assert.Equal(t, exec.ExecutionID, claimedByB.ExecutionID)
	assert.Equal(t, "worker-B", *claimedByB.LockedBy)
}
