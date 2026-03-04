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

func TestHeartbeat_ExtendsLease(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 5, 10, logger)

	tenantID := uuid.New()
	payload := json.RawMessage(`{"task":"heartbeat-test"}`)

	req := domain.CreateExecutionRequest{Payload: payload}
	exec, _, err := svc.CreateExecution(ctx, tenantID, "hb-extend", req)
	require.NoError(t, err)

	claimed, err := svc.ClaimNextExecution(ctx, "worker-hb")
	require.NoError(t, err)
	_, err = svc.UpdateStatus(ctx, claimed.ExecutionID, domain.StatusRunning, claimed.Version)
	require.NoError(t, err)

	updated, err := svc.SendHeartbeat(ctx, exec.ExecutionID, "worker-hb")
	require.NoError(t, err)
	assert.NotNil(t, updated.LockExpiresAt)
	assert.NotNil(t, updated.LastHeartbeatAt)
	assert.True(t, updated.LockExpiresAt.After(time.Now().Add(-1*time.Second)))
}

func TestHeartbeat_LostLease_ReturnsError(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 5, 10, logger)

	tenantID := uuid.New()
	payload := json.RawMessage(`{"task":"lost-lease"}`)

	req := domain.CreateExecutionRequest{Payload: payload}
	_, _, err := svc.CreateExecution(ctx, tenantID, "hb-lost", req)
	require.NoError(t, err)

	claimed, err := svc.ClaimNextExecution(ctx, "worker-original")
	require.NoError(t, err)

	_, err = svc.SendHeartbeat(ctx, claimed.ExecutionID, "worker-wrong")
	assert.Error(t, err)
}
