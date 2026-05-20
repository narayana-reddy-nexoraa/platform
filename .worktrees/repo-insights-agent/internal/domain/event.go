package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event type constants for the execution lifecycle.
const (
	EventExecutionCreated        = "execution.created"
	EventExecutionClaimed        = "execution.claimed"
	EventExecutionStarted        = "execution.started"
	EventExecutionSucceeded      = "execution.succeeded"
	EventExecutionFailed         = "execution.failed"
	EventExecutionRetryScheduled = "execution.retry_scheduled"
	EventExecutionTimedOut       = "execution.timed_out"
	EventExecutionCanceled       = "execution.canceled"
	EventExecutionReclaimed      = "execution.reclaimed"
)

// OutboxEvent is the domain representation of a transactional outbox event.
type OutboxEvent struct {
	EventID        uuid.UUID       `json:"event_id"`
	AggregateType  string          `json:"aggregate_type"`
	AggregateID    uuid.UUID       `json:"aggregate_id"`
	EventType      string          `json:"event_type"`
	Payload        json.RawMessage `json:"payload"`
	Metadata       json.RawMessage `json:"metadata"`
	SequenceNumber int64           `json:"sequence_number"`
	CreatedAt      time.Time       `json:"created_at"`
	Sent           bool            `json:"sent"`
	SentAt         *time.Time      `json:"sent_at,omitempty"`
}

// EventData carries the event-specific payload for execution lifecycle events.
type EventData struct {
	ExecutionID  uuid.UUID `json:"execution_id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	FromStatus   string    `json:"from_status,omitempty"`
	ToStatus     string    `json:"to_status"`
	AttemptCount int32     `json:"attempt_count,omitempty"`
	ErrorCode    string    `json:"error_code,omitempty"`
	WorkerID     string    `json:"worker_id,omitempty"`
}

// EventMetadata carries cross-cutting metadata for outbox events.
type EventMetadata struct {
	TenantID      string `json:"tenant_id"`
	CorrelationID string `json:"correlation_id,omitempty"`
	CausationID   string `json:"causation_id,omitempty"`
	ProducedBy    string `json:"produced_by"`
}

// DeadLetterEvent represents an event that failed processing.
type DeadLetterEvent struct {
	ID            uuid.UUID       `json:"id"`
	EventID       uuid.UUID       `json:"event_id"`
	ConsumerGroup string          `json:"consumer_group"`
	EventType     string          `json:"event_type"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   uuid.UUID       `json:"aggregate_id"`
	Payload       json.RawMessage `json:"payload"`
	Metadata      json.RawMessage `json:"metadata"`
	ErrorMessage  string          `json:"error_message"`
	AttemptCount  int32           `json:"attempt_count"`
	MaxAttempts   int32           `json:"max_attempts"`
	CreatedAt     time.Time       `json:"created_at"`
	LastFailedAt  time.Time       `json:"last_failed_at"`
}

// NewExecutionEvent builds the JSON payload and metadata for an execution lifecycle event.
// It populates EventData from the Execution struct and EventMetadata with the tenant and trigger source.
// correlationID is propagated from the request context to enable end-to-end tracing.
func NewExecutionEvent(eventType string, exec *Execution, fromStatus string, triggeredBy string, correlationID string) (payload, metadata json.RawMessage, err error) {
	data := EventData{
		ExecutionID: exec.ExecutionID,
		TenantID:    exec.TenantID,
		FromStatus:  fromStatus,
		ToStatus:    string(exec.Status),
	}

	if exec.AttemptCount > 0 {
		data.AttemptCount = exec.AttemptCount
	}

	if exec.ErrorCode != nil {
		data.ErrorCode = *exec.ErrorCode
	}

	if exec.LockedBy != nil {
		data.WorkerID = *exec.LockedBy
	}

	payload, err = json.Marshal(data)
	if err != nil {
		return nil, nil, err
	}

	meta := EventMetadata{
		TenantID:      exec.TenantID.String(),
		CorrelationID: correlationID,
		ProducedBy:    triggeredBy,
	}

	metadata, err = json.Marshal(meta)
	if err != nil {
		return nil, nil, err
	}

	return payload, metadata, nil
}
