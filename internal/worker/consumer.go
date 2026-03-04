package worker

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/metrics"
	"github.com/narayana-platform/execution-engine/internal/repository"
)

// EventHandler processes a single event.
type EventHandler func(ctx context.Context, event domain.OutboxEvent) error

// Consumer reads events from a channel and processes them with deduplication.
type Consumer struct {
	repo             repository.ExecutionRepository
	eventChan        <-chan domain.OutboxEvent
	handlers         map[string]EventHandler
	consumerGroup    string
	logger           zerolog.Logger
	startOffset      int64 // offset loaded from DB on startup — used as replay guard
	lastProcessedSeq int64 // running high-water mark — used for offset persistence only
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

	// Load last processed sequence on startup
	offset, err := c.repo.GetConsumerOffset(ctx, c.consumerGroup)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to load consumer offset, starting from 0")
	} else {
		c.startOffset = offset
		c.lastProcessedSeq = offset
		if offset > 0 {
			c.logger.Info().Int64("last_processed_seq", offset).Msg("loaded consumer offset")
		}
	}

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
	start := time.Now()

	// Replay guard: skip events at or below the offset loaded from DB on startup.
	// This prevents reprocessing old events after a restart. During live processing,
	// out-of-order events are handled by the processed_events dedup table.
	if evt.SequenceNumber > 0 && evt.SequenceNumber <= c.startOffset {
		c.logger.Debug().
			Int64("event_seq", evt.SequenceNumber).
			Int64("start_offset", c.startOffset).
			Str("event_id", evt.EventID.String()).
			Msg("skipping already-processed sequence (replay guard)")
		return
	}

	// Gap detection: warn if sequence jumps relative to the high-water mark.
	// Note: false positives are expected during normal out-of-order delivery
	// (e.g., batch arrives as 3,1,2 — gap warning fires on 3 but 1,2 follow).
	if c.lastProcessedSeq > 0 && evt.SequenceNumber > c.lastProcessedSeq+1 {
		c.logger.Warn().
			Int64("event_seq", evt.SequenceNumber).
			Int64("last_processed_seq", c.lastProcessedSeq).
			Int64("gap", evt.SequenceNumber-c.lastProcessedSeq-1).
			Msg("sequence gap detected")
	}

	// Deduplicate via processed_events table
	isNew, err := c.repo.InsertProcessedEvent(ctx, evt.EventID, c.consumerGroup)
	if err != nil {
		c.logger.Error().Err(err).Str("event_id", evt.EventID.String()).Msg("deduplication check failed")
		return
	}
	if !isNew {
		metrics.EventsDeduplicatedTotal.Inc()
		c.logger.Debug().Str("event_id", evt.EventID.String()).Msg("duplicate event skipped")
		c.updateOffset(ctx, evt.SequenceNumber)
		return
	}

	// Route to handler
	handler, ok := c.handlers[evt.EventType]
	if !ok {
		c.logger.Warn().Str("event_type", evt.EventType).Msg("no handler for event type, skipping")
		c.updateOffset(ctx, evt.SequenceNumber)
		return
	}

	if err := handler(ctx, evt); err != nil {
		c.logger.Error().Err(err).
			Str("event_type", evt.EventType).
			Str("event_id", evt.EventID.String()).
			Msg("handler failed, sending to DLQ")

		if dlqErr := c.repo.InsertDLQEvent(ctx, evt, c.consumerGroup, err.Error()); dlqErr != nil {
			c.logger.Error().Err(dlqErr).
				Str("event_id", evt.EventID.String()).
				Msg("failed to insert into DLQ")
		} else {
			metrics.EventsDLQTotal.Inc()
		}
	}

	// EventsProcessedTotal counts total throughput: both successfully handled
	// events and those routed to the DLQ after handler failure.
	metrics.EventsProcessedTotal.Inc()
	metrics.ConsumerProcessingDurationSeconds.Observe(time.Since(start).Seconds())

	c.updateOffset(ctx, evt.SequenceNumber)
}

func (c *Consumer) updateOffset(ctx context.Context, seq int64) {
	if seq > c.lastProcessedSeq {
		c.lastProcessedSeq = seq
		if err := c.repo.UpsertConsumerOffset(ctx, c.consumerGroup, seq); err != nil {
			c.logger.Error().Err(err).Int64("sequence", seq).Msg("failed to update consumer offset")
		}
	}
}

// RegisterHandler overrides the handler for a specific event type.
func (c *Consumer) RegisterHandler(eventType string, handler EventHandler) {
	c.handlers[eventType] = handler
}

func (c *Consumer) handleDefault(ctx context.Context, evt domain.OutboxEvent) error {
	c.logger.Info().
		Str("event_type", evt.EventType).
		Str("aggregate_id", evt.AggregateID.String()).
		Int64("sequence", evt.SequenceNumber).
		Msg("event processed")
	return nil
}
