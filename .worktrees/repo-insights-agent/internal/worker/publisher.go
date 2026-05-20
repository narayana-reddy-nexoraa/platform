package worker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/clock"
	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/metrics"
	"github.com/narayana-platform/execution-engine/internal/repository"
)

const (
	defaultPublisherInterval = 2 * time.Second
	defaultPublisherBatch    = int32(50)
)

// Publisher polls the outbox table and pushes events to an in-process channel.
type Publisher struct {
	repo      repository.ExecutionRepository
	eventChan chan<- domain.OutboxEvent
	logger    zerolog.Logger
	interval  time.Duration
	batchSize int32
	clock     clock.Clock
}

func NewPublisher(repo repository.ExecutionRepository, eventChan chan<- domain.OutboxEvent, logger zerolog.Logger, clk clock.Clock) *Publisher {
	return &Publisher{
		repo:      repo,
		eventChan: eventChan,
		logger:    logger.With().Str("component", "publisher").Logger(),
		interval:  defaultPublisherInterval,
		batchSize: defaultPublisherBatch,
		clock:     clk,
	}
}

func (p *Publisher) Run(ctx context.Context) {
	p.logger.Info().Msg("publisher started")
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info().Msg("publisher stopped")
			return
		case <-ticker.C:
			p.publish(ctx)
		}
	}
}

func (p *Publisher) publish(ctx context.Context) {
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
		select {
		case p.eventChan <- evt:
			sentIDs = append(sentIDs, evt.EventID)
		case <-ctx.Done():
			return
		}
	}

	if len(sentIDs) > 0 {
		if err := p.repo.MarkEventsSent(ctx, sentIDs); err != nil {
			p.logger.Error().Err(err).Int("count", len(sentIDs)).Msg("failed to mark events as sent")
		} else {
			metrics.OutboxPublishDurationSeconds.Observe(p.clock.Now().Sub(start).Seconds())
			metrics.EventsPublishedTotal.Add(float64(len(sentIDs)))
			p.logger.Debug().Int("count", len(sentIDs)).Msg("published events")
		}
	}
}
