package worker

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
)

// EventHandler processes a single event.
type EventHandler func(ctx context.Context, event domain.OutboxEvent) error

// Consumer reads events from a channel and processes them with deduplication.
type Consumer struct {
	repo          repository.ExecutionRepository
	eventChan     <-chan domain.OutboxEvent
	handlers      map[string]EventHandler
	consumerGroup string
	logger        zerolog.Logger
}

func NewConsumer(repo repository.ExecutionRepository, eventChan <-chan domain.OutboxEvent, consumerGroup string, logger zerolog.Logger) *Consumer {
	c := &Consumer{
		repo:          repo,
		eventChan:     eventChan,
		consumerGroup: consumerGroup,
		logger:        logger.With().Str("component", "consumer").Str("group", consumerGroup).Logger(),
	}
	c.handlers = map[string]EventHandler{
		domain.EventExecutionCreated:        c.handleDefault,
		domain.EventExecutionClaimed:        c.handleDefault,
		domain.EventExecutionStarted:        c.handleDefault,
		domain.EventExecutionSucceeded:      c.handleDefault,
		domain.EventExecutionFailed:         c.handleDefault,
		domain.EventExecutionRetryScheduled: c.handleDefault,
		domain.EventExecutionTimedOut:       c.handleDefault,
		domain.EventExecutionCanceled:       c.handleDefault,
		domain.EventExecutionReclaimed:      c.handleDefault,
	}
	return c
}

func (c *Consumer) Run(ctx context.Context) {
	c.logger.Info().Msg("consumer started")
	for {
		select {
		case <-ctx.Done():
			c.logger.Info().Msg("consumer stopped")
			return
		case evt, ok := <-c.eventChan:
			if !ok {
				c.logger.Info().Msg("event channel closed")
				return
			}
			c.process(ctx, evt)
		}
	}
}

func (c *Consumer) process(ctx context.Context, evt domain.OutboxEvent) {
	// Deduplicate
	isNew, err := c.repo.InsertProcessedEvent(ctx, evt.EventID, c.consumerGroup)
	if err != nil {
		c.logger.Error().Err(err).Str("event_id", evt.EventID.String()).Msg("deduplication check failed")
		return
	}
	if !isNew {
		c.logger.Debug().Str("event_id", evt.EventID.String()).Msg("duplicate event skipped")
		return
	}

	// Route to handler
	handler, ok := c.handlers[evt.EventType]
	if !ok {
		c.logger.Warn().Str("event_type", evt.EventType).Msg("no handler for event type, skipping")
		return
	}

	if err := handler(ctx, evt); err != nil {
		c.logger.Error().Err(err).Str("event_type", evt.EventType).Str("event_id", evt.EventID.String()).Msg("handler failed")
	}
}

func (c *Consumer) handleDefault(ctx context.Context, evt domain.OutboxEvent) error {
	c.logger.Info().
		Str("event_type", evt.EventType).
		Str("aggregate_id", evt.AggregateID.String()).
		Int64("sequence", evt.SequenceNumber).
		Msg("event processed")
	return nil
}
