package broker

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/narayana-platform/execution-engine/internal/domain"
)

// Kafka topic constants. All SOP-related events flow through these topics.
const (
	// TopicSOPExecutions carries SOP lifecycle events (started, step completed, finished).
	TopicSOPExecutions = "sop.executions.events"

	// TopicHITLRequests carries human-in-the-loop approval requests.
	TopicHITLRequests = "sop.hitl.requests"

	// TopicHITLResponses carries HITL decisions (approved/rejected/escalated).
	TopicHITLResponses = "sop.hitl.responses"

	// TopicAuditTrail carries immutable compliance audit events.
	TopicAuditTrail = "audit.trail"

	// TopicDLQPrefix is the prefix for dead-letter queue topics.
	TopicDLQPrefix = "dlq."
)

// TopicSpec defines a topic to be created with its partition count and replication factor.
type TopicSpec struct {
	Name              string
	Partitions        int32
	ReplicationFactor int16
}

// DefaultTopics returns the standard set of topics for the Nexoraa platform.
func DefaultTopics() []TopicSpec {
	return []TopicSpec{
		{Name: TopicSOPExecutions, Partitions: 6, ReplicationFactor: 1},
		{Name: TopicHITLRequests, Partitions: 3, ReplicationFactor: 1},
		{Name: TopicHITLResponses, Partitions: 3, ReplicationFactor: 1},
		{Name: TopicAuditTrail, Partitions: 6, ReplicationFactor: 1},
		{Name: TopicDLQPrefix + TopicSOPExecutions, Partitions: 1, ReplicationFactor: 1},
		{Name: TopicDLQPrefix + TopicHITLRequests, Partitions: 1, ReplicationFactor: 1},
		{Name: TopicDLQPrefix + TopicHITLResponses, Partitions: 1, ReplicationFactor: 1},
		{Name: TopicDLQPrefix + TopicAuditTrail, Partitions: 1, ReplicationFactor: 1},
	}
}

// TopicManager handles topic creation and validation via the Kafka admin API.
type TopicManager struct {
	admin  *kadm.Client
	logger zerolog.Logger
}

// NewAdminClient creates a bare Kafka client suitable for admin operations.
func NewAdminClient(cfg KafkaConfig) (*kgo.Client, error) {
	return kgo.NewClient(kgo.SeedBrokers(cfg.Brokers...))
}

// NewTopicManager creates a TopicManager from an existing Kafka client.
func NewTopicManager(client *kgo.Client, logger zerolog.Logger) *TopicManager {
	return &TopicManager{
		admin:  kadm.NewClient(client),
		logger: logger.With().Str("component", "topic-manager").Logger(),
	}
}

// EnsureTopics creates any missing topics from the given specs.
// Existing topics are left untouched (idempotent).
func (tm *TopicManager) EnsureTopics(ctx context.Context, specs []TopicSpec) error {
	// List existing topics first.
	listed, err := tm.admin.ListTopics(ctx)
	if err != nil {
		return fmt.Errorf("list topics: %w", err)
	}

	existing := make(map[string]bool, len(listed))
	for t := range listed {
		existing[t] = true
	}

	for _, spec := range specs {
		if existing[spec.Name] {
			tm.logger.Debug().Str("topic", spec.Name).Msg("topic already exists, skipping")
			continue
		}

		resp, err := tm.admin.CreateTopic(ctx, spec.Partitions, spec.ReplicationFactor, nil, spec.Name)
		if err != nil {
			return fmt.Errorf("create topic %s: %w", spec.Name, err)
		}
		if resp.Err != nil {
			return fmt.Errorf("create topic %s: %w", spec.Name, resp.Err)
		}

		tm.logger.Info().
			Str("topic", spec.Name).
			Int32("partitions", spec.Partitions).
			Msg("topic created")
	}

	return nil
}

// TopicForEvent routes an OutboxEvent to the appropriate Kafka topic.
//
// Strategy: aggregate-type primary routing with event-type prefix fallback.
//
// Primary (aggregate type):
//
//	"execution"     | "sop_execution"  →  sop.executions.events
//	"hitl_request"                     →  sop.hitl.requests
//	"hitl_response"                    →  sop.hitl.responses
//	"audit_trail"                      →  audit.trail
//
// Fallback (event type prefix — handles edge cases where aggregate type is
// missing or generic):
//
//	"sop.*" | "execution.*"  →  sop.executions.events
//	"hitl.request.*"         →  sop.hitl.requests
//	"hitl.response.*"        →  sop.hitl.responses
//	"audit.*"                →  audit.trail
//
// Default: sop.executions.events (the broadest topic; unknown events land here
// rather than being silently dropped).
func TopicForEvent(event domain.OutboxEvent) string {
	// Primary: route by aggregate type (DDD boundary → topic)
	switch event.AggregateType {
	case domain.AggregateExecution, domain.AggregateSOPExecution:
		return TopicSOPExecutions
	case domain.AggregateHITLRequest:
		return TopicHITLRequests
	case domain.AggregateHITLResponse:
		return TopicHITLResponses
	case domain.AggregateAuditTrail:
		return TopicAuditTrail
	}

	// Fallback: route by event type prefix
	et := event.EventType
	switch {
	case strings.HasPrefix(et, "sop.") || strings.HasPrefix(et, "execution."):
		return TopicSOPExecutions
	case strings.HasPrefix(et, "hitl.request"):
		return TopicHITLRequests
	case strings.HasPrefix(et, "hitl.response"):
		return TopicHITLResponses
	case strings.HasPrefix(et, "audit."):
		return TopicAuditTrail
	}

	// Default: catch-all to the broadest topic
	return TopicSOPExecutions
}

// DLQTopic returns the dead-letter queue topic name for a given source topic.
func DLQTopic(sourceTopic string) string {
	return TopicDLQPrefix + sourceTopic
}
