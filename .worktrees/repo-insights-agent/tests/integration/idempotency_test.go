package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
)

func TestIdempotentCreation_NewExecution(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateExecutions(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	tenantID := uuid.New()
	payload := json.RawMessage(`{"order_id":"order-123","amount":99.99}`)
	hash, err := domain.ComputePayloadHash(payload)
	require.NoError(t, err)

	exec, isNew, err := repo.CreateIdempotent(ctx, tenantID, "key-001", 3, payload, hash)
	require.NoError(t, err)
	assert.True(t, isNew, "should be a new creation")
	assert.Equal(t, domain.StatusCreated, exec.Status)
	assert.Equal(t, int32(0), exec.AttemptCount)
	assert.Equal(t, int32(3), exec.MaxAttempts)
	assert.Equal(t, int32(1), exec.Version)
	assert.Equal(t, tenantID, exec.TenantID)
}

func TestIdempotentCreation_DuplicateSamePayload(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateExecutions(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	tenantID := uuid.New()
	payload := json.RawMessage(`{"order_id":"order-123","amount":99.99}`)
	hash, err := domain.ComputePayloadHash(payload)
	require.NoError(t, err)

	// First creation
	exec1, isNew1, err := repo.CreateIdempotent(ctx, tenantID, "key-dup", 3, payload, hash)
	require.NoError(t, err)
	assert.True(t, isNew1)

	// Duplicate with same payload
	exec2, isNew2, err := repo.CreateIdempotent(ctx, tenantID, "key-dup", 3, payload, hash)
	require.NoError(t, err)
	assert.False(t, isNew2, "should NOT be a new creation")
	assert.Equal(t, exec1.ExecutionID, exec2.ExecutionID, "should return the same execution")
}

func TestIdempotentCreation_ConflictDifferentPayload(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateExecutions(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	tenantID := uuid.New()

	payload1 := json.RawMessage(`{"order_id":"order-123","amount":99.99}`)
	hash1, _ := domain.ComputePayloadHash(payload1)

	payload2 := json.RawMessage(`{"order_id":"order-456","amount":50.00}`)
	hash2, _ := domain.ComputePayloadHash(payload2)

	// First creation
	_, _, err := repo.CreateIdempotent(ctx, tenantID, "key-conflict", 3, payload1, hash1)
	require.NoError(t, err)

	// Same key, different payload → conflict
	_, _, err = repo.CreateIdempotent(ctx, tenantID, "key-conflict", 3, payload2, hash2)
	assert.Error(t, err)

	var conflictErr *domain.ErrIdempotencyConflict
	assert.ErrorAs(t, err, &conflictErr)
}

func TestIdempotentCreation_SameKeyDifferentTenants(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateExecutions(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	tenant1 := uuid.New()
	tenant2 := uuid.New()
	payload := json.RawMessage(`{"test":true}`)
	hash, _ := domain.ComputePayloadHash(payload)

	// Same key for different tenants should both succeed
	exec1, isNew1, err := repo.CreateIdempotent(ctx, tenant1, "shared-key", 3, payload, hash)
	require.NoError(t, err)
	assert.True(t, isNew1)

	exec2, isNew2, err := repo.CreateIdempotent(ctx, tenant2, "shared-key", 3, payload, hash)
	require.NoError(t, err)
	assert.True(t, isNew2)

	assert.NotEqual(t, exec1.ExecutionID, exec2.ExecutionID, "different tenants should get different executions")
}
