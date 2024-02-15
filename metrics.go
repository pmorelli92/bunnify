package bunnify

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	queue      = "queue"
	exchange   = "exchange"
	result     = "result"
	routingKey = "routing_key"
)

var (
	eventReceivedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "amqp_events_received",
			Help: "Count of AMQP events received",
		}, []string{queue, routingKey},
	)

	eventWithoutHandlerCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "amqp_events_without_handler",
			Help: "Count of AMQP events without a handler",
		}, []string{queue, routingKey},
	)

	eventNotParsableCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "amqp_events_not_parsable",
			Help: "Count of AMQP events that could not be parsed",
		}, []string{queue, routingKey},
	)

	eventNackCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "amqp_events_nack",
			Help: "Count of AMQP events that were not acknowledged",
		}, []string{queue, routingKey},
	)

	eventAckCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "amqp_events_ack",
			Help: "Count of AMQP events that were acknowledged",
		}, []string{queue, routingKey},
	)

	eventProcessedDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "amqp_events_processed_duration",
			Help:    "Milliseconds taken to process an event",
			Buckets: []float64{100, 200, 500, 1000, 3000, 5000, 10000},
		}, []string{queue, routingKey, result},
	)

	eventPublishSucceed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "amqp_events_publish_succeed",
			Help: "Count of AMQP events that could be published successfully",
		}, []string{exchange, routingKey},
	)

	eventPublishFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "amqp_events_publish_failed",
			Help: "Count of AMQP events that could not be published",
		}, []string{exchange, routingKey},
	)
)

func EventReceived(queue string, routingKey string) {
	eventReceivedCounter.WithLabelValues(queue, routingKey).Inc()
}

func EventWithoutHandler(queue string, routingKey string) {
	eventWithoutHandlerCounter.WithLabelValues(queue, routingKey).Inc()
}

func EventNotParsable(queue string, routingKey string) {
	eventNotParsableCounter.WithLabelValues(queue, routingKey).Inc()
}

func EventNack(queue string, routingKey string, milliseconds int64) {
	eventNackCounter.WithLabelValues(queue, routingKey).Inc()

	eventProcessedDuration.
		WithLabelValues(queue, routingKey, "NACK").
		Observe(float64(milliseconds))
}

func EventAck(queue string, routingKey string, milliseconds int64) {
	eventAckCounter.WithLabelValues(queue, routingKey).Inc()

	eventProcessedDuration.
		WithLabelValues(queue, routingKey, "ACK").
		Observe(float64(milliseconds))
}

func EventPublishSucceed(exchange string, routingKey string) {
	eventPublishSucceed.WithLabelValues(exchange, routingKey).Inc()
}

func EventPublishFailed(exchange string, routingKey string) {
	eventPublishFailed.WithLabelValues(exchange, routingKey).Inc()
}

func InitMetrics(registerer prometheus.Registerer) error {
	collectors := []prometheus.Collector{
		eventReceivedCounter,
		eventAckCounter,
		eventNackCounter,
		eventWithoutHandlerCounter,
		eventNotParsableCounter,
		eventProcessedDuration,
		eventPublishSucceed,
		eventPublishFailed,
	}
	for _, collector := range collectors {
		mv, ok := collector.(metricResetter)
		if ok {
			mv.Reset()
		}
		err := registerer.Register(collector)
		if err != nil && !errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			return err
		}
	}
	return nil
}

// CounterVec, MetricVec and HistogramVec have a Reset func
// in order not to cast to each specific type, metricResetter can
// be used to just get access to the Reset func
type metricResetter interface {
	Reset()
}
