package broker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/narayana-platform/execution-engine/internal/domain"
)

// KafkaProducer publishes outbox events to Kafka topics.
// Events are partitioned by tenant_id extracted from event metadata,
// guaranteeing per-tenant ordering within each topic.
type KafkaProducer struct {
	client *kgo.Client
	logger zerolog.Logger
}

// NewKafkaProducer creates a producer connected to the given Kafka brokers.
func NewKafkaProducer(cfg KafkaConfig, logger zerolog.Logger) (*KafkaProducer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ProduceRequestTimeout(cfg.ProduceTimeout),
		kgo.RequiredAcks(kgo.AllISRAcks()),

		// RecordPartitioner defaults to StickyKeyPartitioner which uses the
		// record's Key field to determine the partition — exactly what we want
		// for tenant-based ordering.
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}

	return &KafkaProducer{
		client: client,
		logger: logger.With().Str("component", "kafka-producer").Logger(),
	}, nil
}

// Publish serializes the event as JSON and sends it to the given topic.
// The partition key is the tenant_id from event metadata, ensuring all
// events for a tenant land on the same partition (preserving ordering).
func (p *KafkaProducer) Publish(ctx context.Context, topic string, event domain.OutboxEvent) error {
	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	key := extractPartitionKey(event)

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
		Headers: []kgo.RecordHeader{
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "aggregate_type", Value: []byte(event.AggregateType)},
			{Key: "event_id", Value: []byte(event.EventID.String())},
		},
	}

	// Synchronous produce — waits for broker ack.
	results := p.client.ProduceSync(ctx, record)
	if err := results.FirstErr(); err != nil {
		kafkaProduceErrorsTotal.Inc()
		return fmt.Errorf("produce to %s: %w", topic, err)
	}

	kafkaProducedTotal.WithLabelValues(topic).Inc()
	p.logger.Debug().
		Str("topic", topic).
		Str("event_id", event.EventID.String()).
		Str("partition_key", key).
		Msg("event published to kafka")

	return nil
}

// Close flushes any buffered records and shuts down the producer.
func (p *KafkaProducer) Close() error {
	p.client.Close()
	p.logger.Info().Msg("kafka producer closed")
	return nil
}

// extractPartitionKey pulls the tenant_id from event metadata for partition routing.
// Falls back to the aggregate_id if metadata is missing or unparseable.
func extractPartitionKey(event domain.OutboxEvent) string {
	var meta domain.EventMetadata
	if err := json.Unmarshal(event.Metadata, &meta); err == nil && meta.TenantID != "" {
		return meta.TenantID
	}
	return event.AggregateID.String()
}
