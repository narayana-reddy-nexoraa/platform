package broker

import (
	"strings"
	"time"

	"github.com/narayana-platform/execution-engine/internal/config"
)

// KafkaConfig holds Kafka connection and tuning parameters.
type KafkaConfig struct {
	// Brokers is a comma-separated list of Kafka bootstrap servers.
	Brokers []string

	// GroupID is the consumer group identifier.
	GroupID string

	// ProduceTimeout is the max time to wait for a produce ack.
	ProduceTimeout time.Duration

	// SessionTimeout is the consumer group session timeout.
	SessionTimeout time.Duration

	// MaxPollRecords limits how many records a single poll returns.
	MaxPollRecords int
}

// KafkaConfigFromApp builds a KafkaConfig from the application config.
func KafkaConfigFromApp(cfg *config.Config) KafkaConfig {
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	return KafkaConfig{
		Brokers:        brokers,
		GroupID:        cfg.KafkaGroupID,
		ProduceTimeout: 10 * time.Second,
		SessionTimeout: 30 * time.Second,
		MaxPollRecords: 500,
	}
}
