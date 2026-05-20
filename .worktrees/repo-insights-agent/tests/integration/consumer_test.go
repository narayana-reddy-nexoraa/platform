package integration

import (
	"context"
	"encoding/json"
	"os"
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
	"github.com/narayana-platform/execution-engine/internal/worker"
)

// TestConsumer_ProcessesNewEvents verifies that the consumer reads an event from
// the channel, processes it, and records a row in processed_events for
// deduplication.
func TestConsumer_ProcessesNewEvents(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	eventChan := make(chan domain.OutboxEvent, 10)
	consumer := worker.NewConsumer(repo, eventChan, "test-group", logger, clock.RealClock{})

	eventID := uuid.New()
	evt := domain.OutboxEvent{
		EventID:        eventID,
		AggregateType:  "execution",
		AggregateID:    uuid.New(),
		EventType:      domain.EventExecutionSucceeded,
		Payload:        json.RawMessage(`{"test":true}`),
		Metadata:       json.RawMessage(`{"producer":"test"}`),
		SequenceNumber: 1,
		CreatedAt:      time.Now(),
	}

	// Put the event on the channel before starting the consumer.
	eventChan <- evt

	// Start consumer and give it time to process.
	consumerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		consumer.Run(consumerCtx)
		close(done)
	}()

	time.Sleep(1 * time.Second)
	cancel()
	<-done

	// Verify that the event was recorded in processed_events.
	var count int64
	err := testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM processed_events WHERE event_id = $1 AND consumer_group = $2",
		eventID, "test-group",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "processed_events should contain 1 row for the event")
}

// TestConsumer_DeduplicatesEvents verifies that the consumer skips an event that
// has already been recorded in processed_events for the same consumer group.
func TestConsumer_DeduplicatesEvents(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	queries := db.New(testPool)
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	eventID := uuid.New()

	// Pre-insert a processed_events row so the event appears as already handled.
	_, err := queries.InsertProcessedEvent(ctx, db.InsertProcessedEventParams{
		EventID:       eventID,
		ConsumerGroup: "test-group",
	})
	require.NoError(t, err)

	// Build an event with the same ID and send it through the channel.
	eventChan := make(chan domain.OutboxEvent, 10)
	consumer := worker.NewConsumer(repo, eventChan, "test-group", logger, clock.RealClock{})

	evt := domain.OutboxEvent{
		EventID:        eventID,
		AggregateType:  "execution",
		AggregateID:    uuid.New(),
		EventType:      domain.EventExecutionSucceeded,
		Payload:        json.RawMessage(`{"test":true}`),
		Metadata:       json.RawMessage(`{"producer":"test"}`),
		SequenceNumber: 1,
		CreatedAt:      time.Now(),
	}
	eventChan <- evt

	// Start consumer and give it time to process.
	consumerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		consumer.Run(consumerCtx)
		close(done)
	}()

	time.Sleep(1 * time.Second)
	cancel()
	<-done

	// Verify exactly 1 row in processed_events — the pre-inserted one. No
	// duplicate was created because the consumer's deduplication logic skipped it.
	var count int64
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM processed_events WHERE event_id = $1 AND consumer_group = $2",
		eventID, "test-group",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "should still have exactly 1 processed_events row (duplicate skipped)")
}
