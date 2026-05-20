package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
	"github.com/narayana-platform/execution-engine/internal/repository/db"
)

// TestOutbox_AtomicWrite_CompleteWithOutbox verifies that CompleteWithOutbox
// atomically writes the execution state change, transition record, and outbox
// event within a single transaction.
func TestOutbox_AtomicWrite_CompleteWithOutbox(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	queries := db.New(testPool)

	tenantID := uuid.New()

	// Step 1: Create an execution
	created, err := queries.CreateExecution(ctx, db.CreateExecutionParams{
		TenantID:       tenantID,
		IdempotencyKey: "outbox-atomic-test-" + uuid.NewString(),
		MaxAttempts:    3,
		Payload:        []byte(`{"test":true}`),
		PayloadHash:    "testhash",
	})
	require.NoError(t, err)

	// Step 2: Claim it
	claimed, err := queries.ClaimExecution(ctx, db.ClaimExecutionParams{
		ExecutionID: created.ExecutionID,
		LockedBy:    pgtype.Text{String: "worker-test", Valid: true},
		Column3:     int32(30),
		Version:     created.Version,
	})
	require.NoError(t, err)

	// Step 3: Transition to RUNNING
	running, err := queries.UpdateExecutionStatus(ctx, db.UpdateExecutionStatusParams{
		ExecutionID: claimed.ExecutionID,
		Status:      db.ExecutionStatusRUNNING,
		Version:     claimed.Version,
	})
	require.NoError(t, err)
	assert.Equal(t, db.ExecutionStatusRUNNING, running.Status)

	// Step 4: Complete with outbox (atomic: status + transition + outbox event)
	completed, err := repo.CompleteWithOutbox(ctx, running.ExecutionID, "worker-test", running.Version)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSucceeded, completed.Status)

	// Step 5: Verify outbox event was written
	events, err := queries.ListOutboxEventsByAggregate(ctx, db.ListOutboxEventsByAggregateParams{
		AggregateType: "execution",
		AggregateID:   created.ExecutionID,
	})
	require.NoError(t, err)
	require.Len(t, events, 1, "expected exactly 1 outbox event")
	assert.True(t, strings.Contains(events[0].EventType, "succeeded"),
		"event type should contain 'succeeded', got: %s", events[0].EventType)
	assert.False(t, events[0].Sent, "outbox event should not be marked as sent yet")

	// Step 6: Verify transition was written
	transitions, err := queries.ListTransitionsByExecution(ctx, created.ExecutionID)
	require.NoError(t, err)
	require.Len(t, transitions, 1, "expected exactly 1 transition")
	assert.Equal(t, db.ExecutionStatusRUNNING, transitions[0].FromStatus)
	assert.Equal(t, db.ExecutionStatusSUCCEEDED, transitions[0].ToStatus)
}

// TestOutbox_RollbackOnVersionConflict verifies that when CompleteWithOutbox
// fails due to an optimistic lock conflict (wrong version), the entire
// transaction is rolled back — no outbox event and no transition record.
func TestOutbox_RollbackOnVersionConflict(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	queries := db.New(testPool)

	tenantID := uuid.New()

	// Step 1: Create an execution
	created, err := queries.CreateExecution(ctx, db.CreateExecutionParams{
		TenantID:       tenantID,
		IdempotencyKey: "outbox-rollback-test-" + uuid.NewString(),
		MaxAttempts:    3,
		Payload:        []byte(`{"test":true}`),
		PayloadHash:    "testhash",
	})
	require.NoError(t, err)

	// Step 2: Claim it
	claimed, err := queries.ClaimExecution(ctx, db.ClaimExecutionParams{
		ExecutionID: created.ExecutionID,
		LockedBy:    pgtype.Text{String: "worker-test", Valid: true},
		Column3:     int32(30),
		Version:     created.Version,
	})
	require.NoError(t, err)

	// Step 3: Transition to RUNNING
	running, err := queries.UpdateExecutionStatus(ctx, db.UpdateExecutionStatusParams{
		ExecutionID: claimed.ExecutionID,
		Status:      db.ExecutionStatusRUNNING,
		Version:     claimed.Version,
	})
	require.NoError(t, err)

	// Step 4: Attempt CompleteWithOutbox with WRONG version (should fail)
	wrongVersion := running.Version + 999
	_, err = repo.CompleteWithOutbox(ctx, running.ExecutionID, "worker-test", wrongVersion)
	require.Error(t, err, "expected error due to version conflict")

	var optimisticLockErr *domain.ErrOptimisticLock
	assert.ErrorAs(t, err, &optimisticLockErr, "error should be ErrOptimisticLock")

	// Step 5: Verify no outbox events were written (transaction rolled back)
	events, err := queries.ListOutboxEventsByAggregate(ctx, db.ListOutboxEventsByAggregateParams{
		AggregateType: "execution",
		AggregateID:   created.ExecutionID,
	})
	require.NoError(t, err)
	assert.Empty(t, events, "outbox events should be empty after rollback")

	// Step 6: Verify no transitions were written (transaction rolled back)
	transitions, err := queries.ListTransitionsByExecution(ctx, created.ExecutionID)
	require.NoError(t, err)
	assert.Empty(t, transitions, "transitions should be empty after rollback")
}
