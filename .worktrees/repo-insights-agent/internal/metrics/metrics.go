// Package metrics defines all Prometheus metrics for the execution engine.
// Metrics are auto-registered with the default Prometheus registry via promauto.
// This package is safe to import from any layer without introducing import cycles.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "execution_engine"

// --- Execution lifecycle counters ---

// ExecutionsCreatedTotal counts executions that have been created.
var ExecutionsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "executions_created_total",
	Help:      "Total number of executions created.",
})

// ExecutionsClaimedTotal counts executions that have been claimed by a worker.
var ExecutionsClaimedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "executions_claimed_total",
	Help:      "Total number of executions claimed by workers.",
})

// ExecutionsSucceededTotal counts executions that completed successfully.
var ExecutionsSucceededTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "executions_succeeded_total",
	Help:      "Total number of executions that succeeded.",
})

// ExecutionsFailedTotal counts executions that failed.
var ExecutionsFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "executions_failed_total",
	Help:      "Total number of executions that failed.",
})

// ExecutionsRetriedTotal counts executions that were retried after failure.
var ExecutionsRetriedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "executions_retried_total",
	Help:      "Total number of executions retried after failure.",
})

// ExecutionsTimedOutTotal counts executions that timed out.
var ExecutionsTimedOutTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "executions_timed_out_total",
	Help:      "Total number of executions that timed out.",
})

// --- Heartbeat and lease counters ---

// HeartbeatsTotal counts heartbeat attempts, partitioned by status (success|failure).
var HeartbeatsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "heartbeats_total",
	Help:      "Total number of heartbeat attempts by status.",
}, []string{"status"})

// LeasesReclaimedTotal counts leases that were reclaimed from expired workers.
var LeasesReclaimedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "leases_reclaimed_total",
	Help:      "Total number of leases reclaimed from expired workers.",
})

// --- Event / outbox counters ---

// EventsPublishedTotal counts events published to the outbox.
var EventsPublishedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "events_published_total",
	Help:      "Total number of events published to the outbox.",
})

// EventsProcessedTotal counts events consumed and processed.
var EventsProcessedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "events_processed_total",
	Help:      "Total number of events consumed and processed.",
})

// EventsDeduplicatedTotal counts events that were skipped due to deduplication.
var EventsDeduplicatedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "events_deduplicated_total",
	Help:      "Total number of events skipped due to deduplication.",
})

// EventsDLQTotal counts events sent to the dead-letter queue.
var EventsDLQTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "events_dlq_total",
	Help:      "Total number of events sent to the dead-letter queue.",
})

// --- Gauges ---

// ExecutionsActiveCount tracks the current number of actively running executions.
var ExecutionsActiveCount = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: namespace,
	Name:      "executions_active_count",
	Help:      "Current number of actively running executions.",
})

// ExecutionsPendingCount tracks the current number of executions waiting to be claimed.
var ExecutionsPendingCount = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: namespace,
	Name:      "executions_pending_count",
	Help:      "Current number of executions waiting to be claimed.",
})

// OutboxUnsentCount tracks the current number of unsent events in the outbox.
var OutboxUnsentCount = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: namespace,
	Name:      "outbox_unsent_count",
	Help:      "Current number of unsent events in the outbox.",
})

// --- Histograms ---

// latencyBuckets provides fine-grained buckets from 5ms to 10s, suitable for
// measuring internal operation latencies (heartbeats, DB queries, publishes).
var latencyBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

// HTTPRequestDurationSeconds observes the duration of HTTP requests,
// partitioned by method and status code.
var HTTPRequestDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: namespace,
	Name:      "http_request_duration_seconds",
	Help:      "Duration of HTTP requests in seconds.",
	Buckets:   latencyBuckets,
}, []string{"method", "status"})

// ExecutionDurationSeconds observes the total wall-clock time of an execution
// from claim to terminal state (succeeded, failed, or timed out).
var ExecutionDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
	Namespace: namespace,
	Name:      "execution_duration_seconds",
	Help:      "Wall-clock duration of an execution from claim to completion.",
	Buckets:   prometheus.DefBuckets,
})

// QueueWaitSeconds observes how long an execution waited in the queue before
// being claimed by a worker.
var QueueWaitSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
	Namespace: namespace,
	Name:      "queue_wait_seconds",
	Help:      "Time an execution waited in the queue before being claimed.",
	Buckets:   prometheus.DefBuckets,
})

// ClaimQueryDurationSeconds observes the latency of the database query used
// to find and claim claimable executions.
var ClaimQueryDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
	Namespace: namespace,
	Name:      "claim_query_duration_seconds",
	Help:      "Latency of the claim query against the database.",
	Buckets:   latencyBuckets,
})

// HeartbeatDurationSeconds observes the latency of a single heartbeat
// (lease renewal) round-trip.
var HeartbeatDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
	Namespace: namespace,
	Name:      "heartbeat_duration_seconds",
	Help:      "Latency of a single heartbeat lease-renewal round-trip.",
	Buckets:   latencyBuckets,
})

// OutboxPublishDurationSeconds observes the latency of publishing a batch of
// events from the outbox to the message broker.
var OutboxPublishDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
	Namespace: namespace,
	Name:      "outbox_publish_duration_seconds",
	Help:      "Latency of publishing events from the outbox.",
	Buckets:   latencyBuckets,
})

// ConsumerProcessingDurationSeconds observes the latency of processing a
// single consumed event.
var ConsumerProcessingDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
	Namespace: namespace,
	Name:      "consumer_processing_duration_seconds",
	Help:      "Latency of processing a single consumed event.",
	Buckets:   latencyBuckets,
})

// Handler returns an http.Handler that serves the Prometheus metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
