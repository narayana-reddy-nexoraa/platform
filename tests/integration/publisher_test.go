package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
	"github.com/narayana-platform/execution-engine/internal/repository/db"
	"github.com/narayana-platform/execution-engine/internal/worker"
)

// TestPublisher_PublishesUnsentEvents verifies that the publisher fetches unsent
// outbox events from the database, pushes them onto the event channel, and marks
// them as sent.
func TestPublisher_PublishesUnsentEvents(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	queries := db.New(testPool)
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Insert 3 outbox events directly into the DB.
	for i := 0; i < 3; i++ {
		err := queries.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
			AggregateType: "execution",
			AggregateID:   uuid.New(),
			EventType:     "execution.succeeded",
			Payload:       []byte(`{"test":true}`),
			Metadata:      []byte(`{"producer":"test"}`),
		})
		require.NoError(t, err, "inserting outbox event %d", i)
	}

	// Verify 3 unsent events exist.
	unsent, err := queries.CountUnsentEvents(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(3), unsent, "should have 3 unsent events before publisher runs")

	// Create buffered channel and publisher.
	eventChan := make(chan domain.OutboxEvent, 10)
	pub := worker.NewPublisher(repo, eventChan, logger)

	// Run publisher in a goroutine; cancel after enough time for one tick (default 2s).
	pubCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		pub.Run(pubCtx)
		close(done)
	}()

	// Wait for at least one tick cycle to complete.
	time.Sleep(3 * time.Second)
	cancel()
	<-done

	// Drain channel and count events received.
	var received []domain.OutboxEvent
	for {
		select {
		case evt := <-eventChan:
			received = append(received, evt)
		default:
			goto drained
		}
	}
drained:

	assert.Len(t, received, 3, "publisher should have pushed 3 events onto the channel")

	// All events should now be marked as sent in the DB.
	unsent, err = queries.CountUnsentEvents(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), unsent, "all events should be marked as sent")
}

// TestPublisher_SkipsWhenNoEvents verifies that the publisher does not push
// anything onto the channel when there are no unsent outbox events.
func TestPublisher_SkipsWhenNoEvents(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateAll(ctx))

	repo := repository.NewPostgresExecutionRepository(testPool)
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	eventChan := make(chan domain.OutboxEvent, 10)
	pub := worker.NewPublisher(repo, eventChan, logger)

	// Run publisher for one tick then cancel.
	pubCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		pub.Run(pubCtx)
		close(done)
	}()

	time.Sleep(3 * time.Second)
	cancel()
	<-done

	// Channel should be empty — no events to publish.
	select {
	case evt := <-eventChan:
		t.Fatalf("expected no events on channel, got: %+v", evt)
	default:
		// success — channel is empty
	}
}
