package broker

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ConsumerLagMonitor periodically checks consumer group lag and exposes
// it as Prometheus metrics. This enables alerting on growing lag before
// it becomes a production issue.
type ConsumerLagMonitor struct {
	admin    *kadm.Client
	groupID  string
	topics   []string
	interval time.Duration
	logger   zerolog.Logger
}

// NewConsumerLagMonitor creates a lag monitor for the given consumer group.
func NewConsumerLagMonitor(client *kgo.Client, groupID string, topics []string, logger zerolog.Logger) *ConsumerLagMonitor {
	return &ConsumerLagMonitor{
		admin:    kadm.NewClient(client),
		groupID:  groupID,
		topics:   topics,
		interval: 15 * time.Second,
		logger:   logger.With().Str("component", "lag-monitor").Logger(),
	}
}

// Run polls consumer lag on a timer. Blocks until ctx is cancelled.
func (m *ConsumerLagMonitor) Run(ctx context.Context) {
	m.logger.Info().
		Str("group_id", m.groupID).
		Strs("topics", m.topics).
		Msg("consumer lag monitor started")

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info().Msg("consumer lag monitor stopped")
			return
		case <-ticker.C:
			m.collectLag(ctx)
		}
	}
}

func (m *ConsumerLagMonitor) collectLag(ctx context.Context) {
	// Fetch committed offsets for the consumer group
	offsets, err := m.admin.FetchOffsets(ctx, m.groupID)
	if err != nil {
		m.logger.Error().Err(err).Msg("failed to fetch consumer offsets")
		return
	}
	if offsets.Error() != nil {
		m.logger.Error().Err(offsets.Error()).Msg("offset fetch returned error")
		return
	}

	// Fetch end offsets (latest) for each topic
	endOffsets, err := m.admin.ListEndOffsets(ctx, m.topics...)
	if err != nil {
		m.logger.Error().Err(err).Msg("failed to fetch end offsets")
		return
	}

	var totalLag int64

	endOffsets.Each(func(lo kadm.ListedOffset) {
		topic := lo.Topic
		partition := lo.Partition

		// Get committed offset for this partition
		committed, exists := offsets.Lookup(topic, partition)
		if !exists {
			// No committed offset — lag is the entire partition
			lag := lo.Offset
			consumerLagGauge.WithLabelValues(topic, fmt.Sprintf("%d", partition), m.groupID).Set(float64(lag))
			totalLag += lag
			return
		}

		lag := lo.Offset - committed.At
		if lag < 0 {
			lag = 0
		}

		consumerLagGauge.WithLabelValues(topic, fmt.Sprintf("%d", partition), m.groupID).Set(float64(lag))
		totalLag += lag
	})

	consumerTotalLagGauge.WithLabelValues(m.groupID).Set(float64(totalLag))

	if totalLag > 0 {
		m.logger.Debug().
			Int64("total_lag", totalLag).
			Str("group_id", m.groupID).
			Msg("consumer lag collected")
	}
}
