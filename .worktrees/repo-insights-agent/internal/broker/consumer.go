package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/narayana-platform/execution-engine/internal/domain"
)

// KafkaConsumer reads events from Kafka consumer groups and dispatches to handlers.
type KafkaConsumer struct {
	client  *kgo.Client
	handler EventHandler
	topics  []string
	logger  zerolog.Logger
}

// NewKafkaConsumer creates a consumer that joins the configured consumer group.
func NewKafkaConsumer(cfg KafkaConfig, logger zerolog.Logger) (*KafkaConsumer, error) {
	kc := &KafkaConsumer{
		logger: logger.With().Str("component", "kafka-consumer").Logger(),
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.SessionTimeout(cfg.SessionTimeout),
		kgo.FetchMaxBytes(5*1024*1024), // 5 MB per fetch
		kgo.OnPartitionsAssigned(func(_ context.Context, _ *kgo.Client, assigned map[string][]int32) {
			for topic, partitions := range assigned {
				kc.logger.Info().
					Str("topic", topic).
					Ints32("partitions", partitions).
					Msg("partitions assigned")
			}
		}),
		kgo.OnPartitionsRevoked(func(_ context.Context, c *kgo.Client, revoked map[string][]int32) {
			// Commit offsets for revoked partitions before giving them up.
			if err := c.CommitMarkedOffsets(context.Background()); err != nil {
				kc.logger.Error().Err(err).Msg("failed to commit offsets on revoke")
			}
			for topic, partitions := range revoked {
				kc.logger.Info().
					Str("topic", topic).
					Ints32("partitions", partitions).
					Msg("partitions revoked")
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}

	kc.client = client
	return kc, nil
}

// Subscribe registers the handler and topics. Must be called before Run.
func (kc *KafkaConsumer) Subscribe(topics []string, handler EventHandler) {
	kc.topics = topics
	kc.handler = handler
	kc.client.AddConsumeTopics(topics...)
	kc.logger.Info().Strs("topics", topics).Msg("subscribed to topics")
}

// Run polls Kafka in a loop, dispatching each record to the handler.
// Blocks until ctx is cancelled. Commits offsets after each batch.
func (kc *KafkaConsumer) Run(ctx context.Context) error {
	if kc.handler == nil {
		return fmt.Errorf("no handler registered; call Subscribe before Run")
	}

	kc.logger.Info().Msg("kafka consumer started")
	defer kc.logger.Info().Msg("kafka consumer stopped")

	for {
		fetches := kc.client.PollFetches(ctx)

		if ctx.Err() != nil {
			return nil
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				kc.logger.Error().
					Str("topic", fe.Topic).
					Int32("partition", fe.Partition).
					Err(fe.Err).
					Msg("fetch error")
				kafkaConsumeErrorsTotal.Inc()
			}
		}

		fetches.EachRecord(func(record *kgo.Record) {
			start := time.Now()

			event, err := deserializeRecord(record)
			if err != nil {
				kc.logger.Error().
					Err(err).
					Str("topic", record.Topic).
					Int64("offset", record.Offset).
					Msg("failed to deserialize record, skipping")
				kafkaConsumeErrorsTotal.Inc()
				// Mark offset so we don't re-read this bad record forever.
				kc.client.MarkCommitRecords(record)
				return
			}

			if err := kc.handler(ctx, event); err != nil {
				kc.logger.Error().
					Err(err).
					Str("event_id", event.EventID.String()).
					Str("topic", record.Topic).
					Msg("handler error")
				kafkaConsumeErrorsTotal.Inc()
			}

			kafkaConsumedTotal.WithLabelValues(record.Topic).Inc()
			kafkaConsumeLatencySeconds.Observe(time.Since(start).Seconds())

			// Mark this record's offset for the next commit.
			kc.client.MarkCommitRecords(record)
		})

		// Commit all marked offsets after processing the batch.
		if err := kc.client.CommitMarkedOffsets(ctx); err != nil {
			kc.logger.Error().Err(err).Msg("failed to commit offsets")
		}
	}
}

// Close shuts down the consumer and commits final offsets.
func (kc *KafkaConsumer) Close() error {
	kc.client.Close()
	kc.logger.Info().Msg("kafka consumer closed")
	return nil
}

// deserializeRecord converts a Kafka record into a domain OutboxEvent.
func deserializeRecord(record *kgo.Record) (domain.OutboxEvent, error) {
	var event domain.OutboxEvent
	if err := json.Unmarshal(record.Value, &event); err != nil {
		return event, fmt.Errorf("unmarshal record at offset %d: %w", record.Offset, err)
	}
	return event, nil
}
