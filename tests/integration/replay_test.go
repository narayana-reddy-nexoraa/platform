package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
	"github.com/narayana-platform/execution-engine/internal/worker"
)

// pollProcessedCount polls until the processed_events count for a consumer group reaches the expected value.
func pollProcessedCount(t *testing.T, ctx context.Context, group string, expected int64) {
	t.Helper()
	require.Eventually(t, func() bool {
		var count int64
		testPool.QueryRow(ctx, "SELECT COUNT(*) FROM processed_events WHERE consumer_group = $1", group).Scan(&count)
		return count >= expected
	}, 10*time.Second, 100*time.Millisecond, "expected %d processed events for group %s", expected, group)
}

// pollOutboxAllSent polls until all outbox events are marked as sent.
func pollOutboxAllSent(t *testing.T, ctx context.Context) {
	t.Helper()
	require.Eventually(t, func() bool {
		var unsent int64
		testPool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events WHERE sent = FALSE").Scan(&unsent)
		return unsent == 0
	}, 10*time.Second, 100*time.Millisecond, "all outbox events should be marked sent")
}

// pollDLQCount polls until the DLQ count for a consumer group reaches the expected value.
func pollDLQCount(t *testing.T, ctx context.Context, group string, expected int64) {
	t.Helper()
	require.Eventually(t, func() bool {
		var count int64
		testPool.QueryRow(ctx, "SELECT COUNT(*) FROM dead_letter_events WHERE consumer_group = $1", group).Scan(&count)
		return count >= expected
	}, 10*time.Second, 100*time.Millisecond, "expected %d DLQ events for group %s", expected, group)
}

// TestReplay_OutOfOrderEvents verifies sequence tracking and replay guard.
// Round 1 sends events out of order (seq 3, 1, 2) and expects all 3 to be
// processed with the offset set to the highest sequence (3).
// Round 2 replays seq 2 with a new consumer on the same group and verifies it
// is skipped by the offset guard.
func TestReplay_OutOfOrderEvents(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Round 1: send events out of order (seq 3, 1, 2)
	eventChan := make(chan domain.OutboxEvent, 10)
	consumer := worker.NewConsumer(repo, eventChan, "ooo-group", logger)

	events := []domain.OutboxEvent{
		{EventID: uuid.New(), AggregateType: "execution", AggregateID: uuid.New(),
			EventType: domain.EventExecutionCreated, Payload: json.RawMessage(`{}`),
			Metadata: json.RawMessage(`{}`), SequenceNumber: 3},
		{EventID: uuid.New(), AggregateType: "execution", AggregateID: uuid.New(),
			EventType: domain.EventExecutionClaimed, Payload: json.RawMessage(`{}`),
			Metadata: json.RawMessage(`{}`), SequenceNumber: 1},
		{EventID: uuid.New(), AggregateType: "execution", AggregateID: uuid.New(),
			EventType: domain.EventExecutionSucceeded, Payload: json.RawMessage(`{}`),
			Metadata: json.RawMessage(`{}`), SequenceNumber: 2},
	}

	for _, evt := range events {
		eventChan <- evt
	}

	consumerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		consumer.Run(consumerCtx)
		close(done)
	}()

	pollProcessedCount(t, ctx, "ooo-group", 3)
	cancel()
	<-done

	// All 3 events processed
	var count int64
	err := testPool.QueryRow(ctx, "SELECT COUNT(*) FROM processed_events WHERE consumer_group = 'ooo-group'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count, "all 3 events should be processed")

	// Offset should be 3 (highest)
	var lastSeq int64
	err = testPool.QueryRow(ctx, "SELECT last_processed_seq FROM consumer_offsets WHERE consumer_group = 'ooo-group'").Scan(&lastSeq)
	require.NoError(t, err)
	assert.Equal(t, int64(3), lastSeq, "offset should be 3")

	// Round 2: new consumer with same group -- send seq 2 again (replay)
	eventChan2 := make(chan domain.OutboxEvent, 10)
	consumer2 := worker.NewConsumer(repo, eventChan2, "ooo-group", logger)
	eventChan2 <- events[2] // seq=2

	consumerCtx2, cancel2 := context.WithCancel(ctx)
	done2 := make(chan struct{})
	go func() {
		consumer2.Run(consumerCtx2)
		close(done2)
	}()
	// Give the consumer time to process (or skip) the replayed event
	time.Sleep(500 * time.Millisecond)
	cancel2()
	<-done2

	// Still 3 (replayed event was skipped by offset guard)
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM processed_events WHERE consumer_group = 'ooo-group'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count, "replayed event should be skipped")
}

// TestReplay_DLQCapture verifies that a failed handler causes the event to land
// in the dead_letter_events table while still being recorded in processed_events.
func TestReplay_DLQCapture(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	eventChan := make(chan domain.OutboxEvent, 10)
	consumer := worker.NewConsumer(repo, eventChan, "dlq-group", logger)

	// Register a failing handler
	consumer.RegisterHandler(domain.EventExecutionFailed, func(ctx context.Context, evt domain.OutboxEvent) error {
		return fmt.Errorf("simulated handler failure")
	})

	eventID := uuid.New()
	evt := domain.OutboxEvent{
		EventID:        eventID,
		AggregateType:  "execution",
		AggregateID:    uuid.New(),
		EventType:      domain.EventExecutionFailed,
		Payload:        json.RawMessage(`{"test":"dlq"}`),
		Metadata:       json.RawMessage(`{}`),
		SequenceNumber: 1,
	}
	eventChan <- evt

	consumerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		consumer.Run(consumerCtx)
		close(done)
	}()

	pollDLQCount(t, ctx, "dlq-group", 1)
	cancel()
	<-done

	// Event should be in processed_events (it was attempted)
	var processedCount int64
	err := testPool.QueryRow(ctx, "SELECT COUNT(*) FROM processed_events WHERE event_id = $1 AND consumer_group = 'dlq-group'", eventID).Scan(&processedCount)
	require.NoError(t, err)
	assert.Equal(t, int64(1), processedCount, "event should be recorded as processed")

	// Event should be in DLQ
	var dlqCount int64
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM dead_letter_events WHERE event_id = $1 AND consumer_group = 'dlq-group'", eventID).Scan(&dlqCount)
	require.NoError(t, err)
	assert.Equal(t, int64(1), dlqCount, "failed event should be in DLQ")

	// Check error message
	var errMsg string
	err = testPool.QueryRow(ctx, "SELECT error_message FROM dead_letter_events WHERE event_id = $1", eventID).Scan(&errMsg)
	require.NoError(t, err)
	assert.Contains(t, errMsg, "simulated handler failure")
}

// TestReplay_EventReplayFromOutbox marks outbox events as unsent after initial
// processing, re-runs publisher+consumer with the same consumer group, and
// verifies that deduplication prevents double processing.
func TestReplay_EventReplayFromOutbox(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Insert 2 outbox events directly using raw SQL
	aggID := uuid.New()
	for i := 0; i < 2; i++ {
		_, err := testPool.Exec(ctx,
			`INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload, metadata)
			 VALUES ($1, $2, $3, $4, $5)`,
			"execution", aggID, domain.EventExecutionCreated,
			fmt.Sprintf(`{"event":%d}`, i+1), `{"producer":"test"}`)
		require.NoError(t, err)
	}

	// Round 1: publisher + consumer
	eventChan := make(chan domain.OutboxEvent, 10)
	pub := worker.NewPublisher(repo, eventChan, logger)
	consumer := worker.NewConsumer(repo, eventChan, "replay-group", logger)

	ctx1, cancel1 := context.WithCancel(ctx)
	done1 := make(chan struct{})
	go func() {
		pub.Run(ctx1)
		close(done1)
	}()
	doneCons1 := make(chan struct{})
	go func() {
		consumer.Run(ctx1)
		close(doneCons1)
	}()

	pollProcessedCount(t, ctx, "replay-group", 2)
	pollOutboxAllSent(t, ctx)
	cancel1()
	<-done1
	<-doneCons1

	// Verify: 2 events processed
	var count int64
	err := testPool.QueryRow(ctx, "SELECT COUNT(*) FROM processed_events WHERE consumer_group = 'replay-group'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "2 events should be processed in round 1")

	// All sent
	var unsent int64
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events WHERE sent = FALSE").Scan(&unsent)
	require.NoError(t, err)
	assert.Equal(t, int64(0), unsent, "all events should be marked sent")

	// Mark as unsent again (simulate replay scenario)
	_, err = testPool.Exec(ctx, "UPDATE outbox_events SET sent = FALSE, sent_at = NULL")
	require.NoError(t, err)

	// Round 2: new publisher + consumer (same consumer group)
	eventChan2 := make(chan domain.OutboxEvent, 10)
	pub2 := worker.NewPublisher(repo, eventChan2, logger)
	consumer2 := worker.NewConsumer(repo, eventChan2, "replay-group", logger)

	ctx2, cancel2 := context.WithCancel(ctx)
	done2 := make(chan struct{})
	go func() {
		pub2.Run(ctx2)
		close(done2)
	}()
	doneCons2 := make(chan struct{})
	go func() {
		consumer2.Run(ctx2)
		close(doneCons2)
	}()

	// Wait for publisher to re-send and consumer to process (or dedup)
	pollOutboxAllSent(t, ctx)
	// Small additional wait for consumer to finish processing the deduplicated events
	time.Sleep(500 * time.Millisecond)
	cancel2()
	<-done2
	<-doneCons2

	// Still only 2 processed events (dedup worked)
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM processed_events WHERE consumer_group = 'replay-group'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "dedup should prevent double processing")

	// No DLQ events (no handler failures)
	var dlqCount int64
	err = testPool.QueryRow(ctx, "SELECT COUNT(*) FROM dead_letter_events WHERE consumer_group = 'replay-group'").Scan(&dlqCount)
	require.NoError(t, err)
	assert.Equal(t, int64(0), dlqCount, "no DLQ events expected")
}
