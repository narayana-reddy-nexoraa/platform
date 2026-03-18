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
