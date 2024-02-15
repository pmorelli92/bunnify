package bunnify

import (
	"encoding/json"
)

type consumerOption struct {
	deadLetterQueue string
	exchange        string
	defaultHandler  wrappedHandler
	handlers        map[string]wrappedHandler
	prefetchCount   int
	prefetchSize    int
	quorumQueue     bool
	notificationCh  chan<- Notification
}

// WithBindingToExchange specifies the exchange on which the queue
// will bind for the handlers provided.
func WithBindingToExchange(exchange string) func(*consumerOption) {
	return func(opt *consumerOption) {
		opt.exchange = exchange
	}
}

// WithQoS specifies the prefetch count and size for the consumer.
func WithQoS(prefetchCount, prefetchSize int) func(*consumerOption) {
	return func(opt *consumerOption) {
		opt.prefetchCount = prefetchCount
		opt.prefetchSize = prefetchSize
	}
}

// WithQuorumQueue specifies that the queue to consume will be created as quorum queue.
// Quorum queues are used when data safety is the priority.
func WithQuorumQueue() func(*consumerOption) {
	return func(opt *consumerOption) {
		opt.quorumQueue = true
	}
}

// WithDeadLetterQueue indicates which queue will receive the events
// that were NACKed for this consumer.
func WithDeadLetterQueue(queueName string) func(*consumerOption) {
	return func(opt *consumerOption) {
		opt.deadLetterQueue = queueName
	}
}

// WithDefaultHandler specifies a handler that can be use for any type
// of routing key without a defined handler. This is mostly convenient if you
// don't care about the specific payload of the event, which will be received as a byte array.
func WithDefaultHandler(handler EventHandler[json.RawMessage]) func(*consumerOption) {
	return func(opt *consumerOption) {
		opt.defaultHandler = newWrappedHandler(handler)
	}
}

// WithHandler specifies under which routing key the provided handler will be invoked.
// The routing key indicated here will be bound to the queue if the WithBindingToExchange is supplied.
func WithHandler[T any](routingKey string, handler EventHandler[T]) func(*consumerOption) {
	return func(opt *consumerOption) {
		opt.handlers[routingKey] = newWrappedHandler(handler)
	}
}
