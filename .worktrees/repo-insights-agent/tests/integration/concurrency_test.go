package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
	"github.com/narayana-platform/execution-engine/internal/service"
)

// TestConcurrentIdempotentCreation proves that 50 goroutines creating with the
// same idempotency key produce exactly 1 execution.
func TestConcurrentIdempotentCreation(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateExecutions(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	payload := json.RawMessage(`{"concurrent":"test"}`)

	const goroutines = 50
	var wg sync.WaitGroup
	var newCount atomic.Int32
	var dupeCount atomic.Int32
	var errCount atomic.Int32
	executionIDs := make([]uuid.UUID, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			req := domain.CreateExecutionRequest{Payload: payload}
			exec, isNew, err := svc.CreateExecution(ctx, tenantID, "concurrent-key", req)
			if err != nil {
				errCount.Add(1)
				return
			}
			executionIDs[idx] = exec.ExecutionID
			if isNew {
				newCount.Add(1)
			} else {
				dupeCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int32(0), errCount.Load(), "should have zero errors")
	assert.Equal(t, int32(1), newCount.Load(), "exactly 1 goroutine should create the execution")
	assert.Equal(t, int32(49), dupeCount.Load(), "49 goroutines should get duplicate response")

	// All should reference the same execution ID
	var firstID uuid.UUID
	for _, id := range executionIDs {
		if id != uuid.Nil {
			if firstID == uuid.Nil {
				firstID = id
			}
			assert.Equal(t, firstID, id, "all goroutines should see the same execution ID")
		}
	}
}

// TestConcurrentClaimRace proves that 50 goroutines trying to claim the same
// execution results in exactly 1 winner.
func TestConcurrentClaimRace(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateExecutions(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	payload := json.RawMessage(`{"race":"test"}`)

	// Create a single execution
	req := domain.CreateExecutionRequest{Payload: payload}
	exec, _, err := svc.CreateExecution(ctx, tenantID, "race-key", req)
	require.NoError(t, err)

	const goroutines = 50
	var wg sync.WaitGroup
	var winnerCount atomic.Int32
	var failCount atomic.Int32
	winners := make([]string, 0)
	var mu sync.Mutex

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			workerID := fmt.Sprintf("worker-%d", idx)
			claimed, err := svc.ClaimNextExecution(ctx, workerID)
			if err != nil {
				failCount.Add(1)
				return
			}
			if claimed.ExecutionID == exec.ExecutionID {
				winnerCount.Add(1)
				mu.Lock()
				winners = append(winners, workerID)
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int32(1), winnerCount.Load(), "exactly 1 worker should claim the execution")
	assert.Equal(t, int32(49), failCount.Load(), "49 workers should fail to claim")
	assert.Len(t, winners, 1, "should have exactly 1 winner")
}

// TestHighVolumeThroughput creates 100 executions and runs 10 simulated workers.
// All executions should be claimed exactly once.
func TestHighVolumeThroughput(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateExecutions(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.Nop()
	svc := service.NewExecutionService(repo, 30, 10, logger)

	tenantID := uuid.New()
	const totalExecutions = 100
	const numWorkers = 10

	// Create 100 executions
	for i := 0; i < totalExecutions; i++ {
		payload := json.RawMessage(fmt.Sprintf(`{"job_id":%d}`, i))
		req := domain.CreateExecutionRequest{Payload: payload}
		_, _, err := svc.CreateExecution(ctx, tenantID, fmt.Sprintf("throughput-key-%d", i), req)
		require.NoError(t, err)
	}

	// Run workers in parallel — each claims until no more work
	var wg sync.WaitGroup
	claimedByWorker := make([]int32, numWorkers)
	allClaimed := make([]uuid.UUID, 0)
	var mu sync.Mutex

	wg.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		go func(workerIdx int) {
			defer wg.Done()
			workerID := fmt.Sprintf("throughput-worker-%d", workerIdx)
			for {
				claimed, err := svc.ClaimNextExecution(ctx, workerID)
				if err != nil {
					break // no more work
				}
				claimedByWorker[workerIdx]++
				mu.Lock()
				allClaimed = append(allClaimed, claimed.ExecutionID)
				mu.Unlock()
			}
		}(w)
	}

	wg.Wait()

	// Total claimed should equal total executions
	var totalClaimed int32
	for _, count := range claimedByWorker {
		totalClaimed += count
	}
	assert.Equal(t, int32(totalExecutions), totalClaimed, "all %d executions should be claimed", totalExecutions)

	// No execution should be claimed twice
	seen := make(map[uuid.UUID]bool)
	for _, id := range allClaimed {
		assert.False(t, seen[id], "execution %s was claimed more than once", id)
		seen[id] = true
	}
}
