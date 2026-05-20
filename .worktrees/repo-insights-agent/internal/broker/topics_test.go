package broker

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/narayana-platform/execution-engine/internal/domain"
)

func TestTopicForEvent_AggregateTypePrimary(t *testing.T) {
	tests := []struct {
		name          string
		aggregateType string
		eventType     string
		wantTopic     string
	}{
		{
			name:          "generic execution aggregate → executions topic",
			aggregateType: domain.AggregateExecution,
			eventType:     domain.EventExecutionCreated,
			wantTopic:     TopicSOPExecutions,
		},
		{
			name:          "sop_execution aggregate → executions topic",
			aggregateType: domain.AggregateSOPExecution,
			eventType:     domain.EventSOPStarted,
			wantTopic:     TopicSOPExecutions,
		},
		{
			name:          "hitl_request aggregate → hitl requests topic",
			aggregateType: domain.AggregateHITLRequest,
			eventType:     domain.EventHITLRequestCreated,
			wantTopic:     TopicHITLRequests,
		},
		{
			name:          "hitl_response aggregate → hitl responses topic",
			aggregateType: domain.AggregateHITLResponse,
			eventType:     domain.EventHITLResponseDecided,
			wantTopic:     TopicHITLResponses,
		},
		{
			name:          "audit_trail aggregate → audit topic",
			aggregateType: domain.AggregateAuditTrail,
			eventType:     domain.EventAuditEntryCreated,
			wantTopic:     TopicAuditTrail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := domain.OutboxEvent{
				EventID:       uuid.New(),
				AggregateType: tt.aggregateType,
				AggregateID:   uuid.New(),
				EventType:     tt.eventType,
			}
			assert.Equal(t, tt.wantTopic, TopicForEvent(event))
		})
	}
}

func TestTopicForEvent_EventTypeFallback(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		wantTopic string
	}{
		{
			name:      "execution.* prefix → executions topic",
			eventType: "execution.claimed",
			wantTopic: TopicSOPExecutions,
		},
		{
			name:      "sop.* prefix → executions topic",
			eventType: "sop.execution.step_completed",
			wantTopic: TopicSOPExecutions,
		},
		{
			name:      "hitl.request.* prefix → hitl requests topic",
			eventType: "hitl.request.created",
			wantTopic: TopicHITLRequests,
		},
		{
			name:      "hitl.response.* prefix → hitl responses topic",
			eventType: "hitl.response.decided",
			wantTopic: TopicHITLResponses,
		},
		{
			name:      "audit.* prefix → audit topic",
			eventType: "audit.entry.created",
			wantTopic: TopicAuditTrail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := domain.OutboxEvent{
				EventID:       uuid.New(),
				AggregateType: "unknown", // force fallback to event type
				AggregateID:   uuid.New(),
				EventType:     tt.eventType,
			}
			assert.Equal(t, tt.wantTopic, TopicForEvent(event))
		})
	}
}

func TestTopicForEvent_DefaultFallback(t *testing.T) {
	event := domain.OutboxEvent{
		EventID:       uuid.New(),
		AggregateType: "something_new",
		AggregateID:   uuid.New(),
		EventType:     "something.completely.unknown",
	}
	assert.Equal(t, TopicSOPExecutions, TopicForEvent(event), "unknown events should default to executions topic")
}

func TestTopicForEvent_AggregateTypeWinsOverEventType(t *testing.T) {
	// If aggregate type says "audit_trail" but event type says "execution.*",
	// aggregate type wins because it's the primary routing strategy.
	event := domain.OutboxEvent{
		EventID:       uuid.New(),
		AggregateType: domain.AggregateAuditTrail,
		AggregateID:   uuid.New(),
		EventType:     "execution.created",
	}
	assert.Equal(t, TopicAuditTrail, TopicForEvent(event))
}

func TestDLQTopic(t *testing.T) {
	assert.Equal(t, "dlq.sop.executions.events", DLQTopic(TopicSOPExecutions))
	assert.Equal(t, "dlq.audit.trail", DLQTopic(TopicAuditTrail))
}

func TestDefaultTopics(t *testing.T) {
	topics := DefaultTopics()
	assert.Len(t, topics, 8, "4 primary topics + 4 DLQ topics")

	names := make(map[string]bool)
	for _, ts := range topics {
		names[ts.Name] = true
		assert.Greater(t, ts.Partitions, int32(0))
		assert.Greater(t, ts.ReplicationFactor, int16(0))
	}

	assert.True(t, names[TopicSOPExecutions])
	assert.True(t, names[TopicHITLRequests])
	assert.True(t, names[TopicHITLResponses])
	assert.True(t, names[TopicAuditTrail])
	assert.True(t, names[DLQTopic(TopicSOPExecutions)])
	assert.True(t, names[DLQTopic(TopicAuditTrail)])
}
