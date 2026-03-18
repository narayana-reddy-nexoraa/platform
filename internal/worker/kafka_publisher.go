package worker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/broker"
	"github.com/narayana-platform/execution-engine/internal/clock"
	"github.com/narayana-platform/execution-engine/internal/metrics"
	"github.com/narayana-platform/execution-engine/internal/repository"
)

// KafkaPublisher polls the outbox table and publishes events to Kafka
// via the broker.EventPublisher interface. It replaces the channel-based
// Publisher when EVENT_BUS=kafka.
type KafkaPublisher struct {
	repo      repository.ExecutionRepository
	producer  broker.EventPublisher
	logger    zerolog.Logger
	interval  time.Duration
	batchSize int32
	clock     clock.Clock
}

// NewKafkaPublisher creates a publisher that sends outbox events to Kafka.
func NewKafkaPublisher(repo repository.ExecutionRepository, producer broker.EventPublisher, logger zerolog.Logger, clk clock.Clock) *KafkaPublisher {
	return &KafkaPublisher{
		repo:      repo,
		producer:  producer,
		logger:    logger.With().Str("component", "kafka-publisher").Logger(),
		interval:  defaultPublisherInterval,
		batchSize: defaultPublisherBatch,
		clock:     clk,
	}
}

// Run polls the outbox and publishes to Kafka in a loop.
func (p *KafkaPublisher) Run(ctx context.Context) {
	p.logger.Info().Msg("kafka publisher started")
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info().Msg("kafka publisher stopped")
			return
		case <-ticker.C:
			p.publish(ctx)
		}
	}
}

func (p *KafkaPublisher) publish(ctx context.Context) {
	start := p.clock.Now()

	events, err := p.repo.FetchUnsentEvents(ctx, p.batchSize)
	if err != nil {
		p.logger.Error().Err(err).Msg("failed to fetch unsent events")
		return
	}

	if len(events) == 0 {
		return
	}

	sentIDs := make([]uuid.UUID, 0, len(events))
	for _, evt := range events {
		topic := broker.TopicForEvent(evt)

		if err := p.producer.Publish(ctx, topic, evt); err != nil {
			p.logger.Error().
				Err(err).
				Str("event_id", evt.EventID.String()).
				Str("topic", topic).
				Msg("failed to publish event to kafka, will retry next cycle")
			continue
		}
		sentIDs = append(sentIDs, evt.EventID)
	}

	if len(sentIDs) > 0 {
		if err := p.repo.MarkEventsSent(ctx, sentIDs); err != nil {
			p.logger.Error().Err(err).Int("count", len(sentIDs)).Msg("failed to mark events as sent")
		} else {
			metrics.OutboxPublishDurationSeconds.Observe(p.clock.Now().Sub(start).Seconds())
			metrics.EventsPublishedTotal.Add(float64(len(sentIDs)))
			p.logger.Debug().Int("count", len(sentIDs)).Msg("published events to kafka")
		}
	}
}
