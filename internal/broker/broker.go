// Package broker provides an abstraction over message transports (Go channel vs Kafka).
// The EVENT_BUS config flag selects which implementation is used at runtime.
package broker

import (
	"context"

	"github.com/narayana-platform/execution-engine/internal/domain"
)

// EventPublisher sends outbox events to a message bus.
type EventPublisher interface {
	// Publish sends a single event to the given topic.
	// The implementation is responsible for serialization and partition key selection.
	Publish(ctx context.Context, topic string, event domain.OutboxEvent) error

	// Close flushes pending writes and releases resources.
	Close() error
}

// EventHandler processes a single consumed event. Returning an error
// signals that the event should be retried or sent to the DLQ.
type EventHandler func(ctx context.Context, event domain.OutboxEvent) error

// EventConsumer reads events from a message bus and dispatches to handlers.
type EventConsumer interface {
	// Subscribe registers a handler for a set of topics.
	// The consumer calls handler for each event received.
	Subscribe(topics []string, handler EventHandler)

	// Run starts consuming. Blocks until ctx is cancelled.
	Run(ctx context.Context) error

	// Close commits final offsets and releases resources.
	Close() error
}
