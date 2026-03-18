package broker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const metricsNamespace = "execution_engine"

// --- Producer metrics ---

var kafkaProducedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: metricsNamespace,
	Subsystem: "kafka",
	Name:      "produced_total",
	Help:      "Total number of events produced to Kafka, by topic.",
}, []string{"topic"})

var kafkaProduceErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: metricsNamespace,
	Subsystem: "kafka",
	Name:      "produce_errors_total",
	Help:      "Total number of Kafka produce errors.",
})

// --- Consumer metrics ---

var kafkaConsumedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: metricsNamespace,
	Subsystem: "kafka",
	Name:      "consumed_total",
	Help:      "Total number of events consumed from Kafka, by topic.",
}, []string{"topic"})

var kafkaConsumeErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: metricsNamespace,
	Subsystem: "kafka",
	Name:      "consume_errors_total",
	Help:      "Total number of Kafka consume/processing errors.",
})

var kafkaConsumeLatencySeconds = promauto.NewHistogram(prometheus.HistogramOpts{
	Namespace: metricsNamespace,
	Subsystem: "kafka",
	Name:      "consume_latency_seconds",
	Help:      "Time to process a single consumed Kafka record.",
	Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
})

// --- Topic manager metrics ---

var kafkaTopicsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: metricsNamespace,
	Subsystem: "kafka",
	Name:      "topics_created_total",
	Help:      "Total number of Kafka topics created by the topic manager.",
})

// --- Circuit breaker metrics ---

var circuitBreakerStateGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: metricsNamespace,
	Subsystem: "circuit_breaker",
	Name:      "state",
	Help:      "Current circuit breaker state (1 = active for that state).",
}, []string{"state"})

var circuitBreakerTransitionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: metricsNamespace,
	Subsystem: "circuit_breaker",
	Name:      "transitions_total",
	Help:      "Total circuit breaker state transitions.",
}, []string{"to_state"})

// --- Adaptive consumer metrics ---

var adaptiveRateGauge = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: metricsNamespace,
	Subsystem: "adaptive",
	Name:      "current_rate",
	Help:      "Current event consumption rate (events/sec).",
})

var adaptiveMovingAvgGauge = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: metricsNamespace,
	Subsystem: "adaptive",
	Name:      "moving_average_rate",
	Help:      "Moving average of event consumption rate (events/sec).",
})

var adaptiveThrottledGauge = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: metricsNamespace,
	Subsystem: "adaptive",
	Name:      "throttled",
	Help:      "Whether the adaptive consumer is in throttled mode (1=throttled, 0=normal).",
})

var adaptiveSpikeDetectedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: metricsNamespace,
	Subsystem: "adaptive",
	Name:      "spikes_detected_total",
	Help:      "Total number of traffic spikes detected.",
})

var adaptiveShedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: metricsNamespace,
	Subsystem: "adaptive",
	Name:      "events_shed_total",
	Help:      "Total events dropped due to burst buffer overflow (load shedding).",
})

// --- Consumer lag metrics ---

var consumerLagGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: metricsNamespace,
	Subsystem: "kafka",
	Name:      "consumer_lag",
	Help:      "Consumer group lag per topic/partition.",
}, []string{"topic", "partition", "group"})

var consumerTotalLagGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: metricsNamespace,
	Subsystem: "kafka",
	Name:      "consumer_total_lag",
	Help:      "Total consumer group lag across all partitions.",
}, []string{"group"})
