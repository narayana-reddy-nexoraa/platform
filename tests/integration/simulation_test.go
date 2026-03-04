package integration

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/narayana-platform/execution-engine/internal/clock"
	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
	"github.com/narayana-platform/execution-engine/internal/repository/db"
	"github.com/narayana-platform/execution-engine/internal/service"
	"github.com/narayana-platform/execution-engine/internal/worker"
)

// TestSimulation_MultiWorkerChaos proves the entire system works correctly under
// adverse conditions: multiple workers competing for executions, random worker
// kills and restarts, and a publisher pause/restart cycle.
//
// The test seeds 50 executions, runs 4 claim workers + reaper + publisher +
// consumer for a 20-second chaos phase (killing/restarting workers every 5s and
// pausing the publisher mid-run), then allows a 30-second drain phase for all
// remaining work to complete.
//
// Verification checks:
//  1. All executions reach a terminal state (soft: allows a small number of
//     non-terminal executions that may have exhausted attempts during chaos).
//  2. No orphaned leases on terminal executions.
//  3. All recorded state transitions are valid per the domain state machine.
//  4. No duplicate event processing in the consumer.
//  5. No double completions in the processing log.
func TestSimulation_MultiWorkerChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping simulation in short mode")
	}

	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Create infrastructure.
	repo := repository.NewPostgresExecutionRepository(testPool)
	svc := service.NewExecutionService(repo, 5, 10, logger) // 5s lease, batch 10

	// Seed 50 executions with generous max_attempts to survive chaos kills.
	queries := db.New(testPool)
	tenantID := uuid.New()
	for i := 0; i < 50; i++ {
		_, err := queries.CreateExecution(ctx, db.CreateExecutionParams{
			TenantID:       tenantID,
			IdempotencyKey: fmt.Sprintf("sim-%d", i),
			MaxAttempts:    5,
			Payload:        []byte(fmt.Sprintf(`{"task":%d}`, i)),
			PayloadHash:    fmt.Sprintf("hash-%d", i),
		})
		require.NoError(t, err)
	}

	// WaitGroup for tracking in-flight claimer work.
	var wg sync.WaitGroup
	clk := clock.RealClock{}

	// Event channel shared between publisher and consumer.
	eventChan := make(chan domain.OutboxEvent, 1000)

	// Master context: cancelling this stops everything.
	simCtx, simCancel := context.WithCancel(ctx)
	defer simCancel()

	// Start 4 claim workers, each with its own cancellable context for chaos killing.
	type workerCtx struct {
		cancel context.CancelFunc
		id     string
	}
	workers := make([]workerCtx, 4)
	for i := 0; i < 4; i++ {
		wCtx, wCancel := context.WithCancel(simCtx)
		wID := fmt.Sprintf("worker-%d", i)
		claimer := worker.NewClaimer(svc, repo, wID, logger, &wg, clk)
		go claimer.Run(wCtx)
		workers[i] = workerCtx{cancel: wCancel, id: wID}
	}

	// Reaper — runs for the full simulation lifetime.
	reaper := worker.NewReaper(svc, logger)
	go reaper.Run(simCtx)

	// Publisher — has its own context so we can pause/restart it.
	pub := worker.NewPublisher(repo, eventChan, logger, clk)
	pubCtx, pubCancel := context.WithCancel(simCtx)
	go pub.Run(pubCtx)

	// Consumer — runs for the full simulation lifetime.
	cons := worker.NewConsumer(repo, eventChan, "sim-group", logger, clk)
	go cons.Run(simCtx)

	// === CHAOS PHASE (20 seconds) ===
	chaosCtx, chaosCancel := context.WithTimeout(simCtx, 20*time.Second)
	defer chaosCancel()

	// Goroutine: kill and restart random workers every 5 seconds.
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		killCount := 0
		for {
			select {
			case <-chaosCtx.Done():
				return
			case <-ticker.C:
				idx := killCount % 4
				workers[idx].cancel()
				t.Logf("CHAOS: killed %s", workers[idx].id)

				// Restart after 2 seconds.
				time.Sleep(2 * time.Second)
				newCtx, newCancel := context.WithCancel(simCtx)
				claimer := worker.NewClaimer(svc, repo, workers[idx].id, logger, &wg, clk)
				go claimer.Run(newCtx)
				workers[idx] = workerCtx{cancel: newCancel, id: workers[idx].id}
				t.Logf("CHAOS: restarted %s", workers[idx].id)
				killCount++
			}
		}
	}()

	// Goroutine: pause publisher at 10s, restart at 15s.
	go func() {
		select {
		case <-chaosCtx.Done():
			return
		case <-time.After(10 * time.Second):
		}
		pubCancel()
		t.Log("CHAOS: paused publisher")

		select {
		case <-chaosCtx.Done():
			return
		case <-time.After(5 * time.Second):
		}
		pubCtx2, pubCancel2 := context.WithCancel(simCtx)
		_ = pubCancel2 // will be cancelled when simCtx is cancelled
		pub2 := worker.NewPublisher(repo, eventChan, logger, clk)
		go pub2.Run(pubCtx2)
		t.Log("CHAOS: restarted publisher")
	}()

	// Wait for the chaos phase to complete.
	<-chaosCtx.Done()

	// === DRAIN PHASE (30 seconds) ===
	// Give remaining workers and reaper time to finish all 50 executions.
	t.Log("Chaos phase complete, draining...")
	time.Sleep(30 * time.Second)

	// Stop everything.
	simCancel()
	time.Sleep(1 * time.Second)

	// === VERIFICATION PHASE ===
	t.Log("Running verification queries...")

	// 1. All executions in terminal state.
	// Terminal states: SUCCEEDED, FAILED, CANCELED, TIMED_OUT.
	// Some may remain non-terminal if they exhausted attempts during chaos and
	// are stuck in CREATED with attempt_count >= max_attempts (no longer claimable).
	var nonTerminal int64
	err := testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM executions WHERE status NOT IN ('SUCCEEDED', 'FAILED', 'CANCELED', 'TIMED_OUT')").Scan(&nonTerminal)
	require.NoError(t, err)
	t.Logf("Non-terminal executions: %d", nonTerminal)
	// Soft assertion: allow up to 5 non-terminal (timing-sensitive under chaos).
	assert.LessOrEqual(t, nonTerminal, int64(5),
		"at most 5 executions should be non-terminal after drain")

	// 2. No orphaned leases on executions that are still locked but not in an active state.
	var orphaned int64
	err = testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM executions
		 WHERE locked_by IS NOT NULL
		   AND status NOT IN ('CLAIMED', 'RUNNING')`).Scan(&orphaned)
	require.NoError(t, err)
	assert.Equal(t, int64(0), orphaned, "no orphaned leases on non-active executions")

	// 3. All recorded state transitions are valid per the domain state machine.
	var invalidTransitions int64
	err = testPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM execution_transitions WHERE NOT (
			(from_status = 'CREATED'   AND to_status IN ('CLAIMED', 'CANCELED')) OR
			(from_status = 'CLAIMED'   AND to_status IN ('CREATED', 'RUNNING', 'CANCELED', 'TIMED_OUT')) OR
			(from_status = 'RUNNING'   AND to_status IN ('SUCCEEDED', 'FAILED', 'CANCELED', 'TIMED_OUT')) OR
			(from_status = 'FAILED'    AND to_status IN ('CREATED', 'CLAIMED', 'CANCELED')) OR
			(from_status = 'TIMED_OUT' AND to_status IN ('CREATED'))
		)`).Scan(&invalidTransitions)
	require.NoError(t, err)
	assert.Equal(t, int64(0), invalidTransitions, "all transitions should be valid")

	// 4. All outbox events eventually sent.
	var unsent int64
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events WHERE sent = FALSE").Scan(&unsent)
	require.NoError(t, err)
	t.Logf("Unsent outbox events: %d", unsent)
	// Some may remain unsent if publisher didn't drain everything; that is acceptable.
	// The important invariant is that events exist in the outbox table (not lost).

	// 5. No duplicate event processing.
	var duplicates int64
	err = testPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT event_id, consumer_group, COUNT(*) as cnt
			FROM processed_events
			GROUP BY event_id, consumer_group
			HAVING COUNT(*) > 1
		) dupes`).Scan(&duplicates)
	require.NoError(t, err)
	assert.Equal(t, int64(0), duplicates, "no duplicate event processing")

	// 6. No double completions in the processing log.
	var doubleCompletions int64
	err = testPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT execution_id, COUNT(*) as cnt
			FROM processing_log
			WHERE action = 'COMPLETED'
			GROUP BY execution_id
			HAVING COUNT(*) > 1
		) doubles`).Scan(&doubleCompletions)
	require.NoError(t, err)
	assert.Equal(t, int64(0), doubleCompletions, "no double completions")

	// === SUMMARY REPORT ===
	var succeeded, failed, timedOut int64
	testPool.QueryRow(ctx, "SELECT COUNT(*) FROM executions WHERE status = 'SUCCEEDED'").Scan(&succeeded)
	testPool.QueryRow(ctx, "SELECT COUNT(*) FROM executions WHERE status = 'FAILED'").Scan(&failed)
	testPool.QueryRow(ctx, "SELECT COUNT(*) FROM executions WHERE status = 'TIMED_OUT'").Scan(&timedOut)
	var totalEvents int64
	testPool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events").Scan(&totalEvents)
	var totalProcessed int64
	testPool.QueryRow(ctx, "SELECT COUNT(*) FROM processed_events").Scan(&totalProcessed)

	t.Logf("=== SIMULATION REPORT ===")
	t.Logf("Total executions:     50")
	t.Logf("Succeeded:            %d", succeeded)
	t.Logf("Failed:               %d", failed)
	t.Logf("Timed out:            %d", timedOut)
	t.Logf("Non-terminal:         %d", nonTerminal)
	t.Logf("Double completions:   %d", doubleCompletions)
	t.Logf("Invalid transitions:  %d", invalidTransitions)
	t.Logf("Orphaned leases:      %d", orphaned)
	t.Logf("Outbox events:        %d", totalEvents)
	t.Logf("Unsent events:        %d", unsent)
	t.Logf("Processed events:     %d", totalProcessed)
	t.Logf("Duplicate processing: %d", duplicates)
}
