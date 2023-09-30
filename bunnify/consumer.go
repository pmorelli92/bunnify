package bunnify

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
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

// Consumer is used for consuming to events from an specified queue.
type Consumer struct {
	queueName     string
	initialized   bool
	options       consumerOption
	getNewChannel func() (*amqp.Channel, bool)
}

// NewConsumer creates a consumer for a given queue using the specified connection.
// Information messages such as channel status will be sent to the notification channel
// if it was specified on the connection struct.
// If no QoS is supplied the prefetch count will be of 20.
func (c *Connection) NewConsumer(
	queueName string,
	opts ...func(*consumerOption)) Consumer {

	options := consumerOption{
		notificationCh: c.options.notificationChannel,
		handlers:       make(map[string]wrappedHandler, 0),
		prefetchCount:  20,
		prefetchSize:   0,
	}
	for _, opt := range opts {
		opt(&options)
	}

	return Consumer{
		queueName: queueName,
		options:   options,
		getNewChannel: func() (*amqp.Channel, bool) {
			return c.getNewChannel(NotificationSourceConsumer)
		},
	}
}

// Consume will start consuming events for the indicated queue.
// The first time this function is called it will return error if
// handlers or default handler are not specified and also if queues, exchanges,
// bindings or qos creation don't succeed. In case this function gets called
// recursively due to channel reconnection, the errors will be pushed to
// the notification channel (if one has been indicated in the connection).
func (c Consumer) Consume() error {
	channel, connectionClosed := c.getNewChannel()
	if connectionClosed {
		return fmt.Errorf("connection is already closed by system")
	}

	if !c.initialized {
		if c.options.defaultHandler == nil && len(c.options.handlers) == 0 {
			return fmt.Errorf("no handlers specified")
		}

		if err := c.createExchanges(channel); err != nil {
			return fmt.Errorf("failed to declare exchange: %w", err)
		}

		if err := c.createQueues(channel); err != nil {
			return fmt.Errorf("failed to declare queue: %w", err)
		}

		if err := c.queueBind(channel); err != nil {
			return fmt.Errorf("failed to bind queue: %w", err)
		}

		c.initialized = true
	}

	if err := channel.Qos(c.options.prefetchCount, c.options.prefetchSize, false); err != nil {
		return fmt.Errorf("failed to set qos: %w", err)
	}

	deliveries, err := channel.Consume(c.queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to establish consuming from queue: %w", err)
	}

	go func() {
		for delivery := range deliveries {
			startTime := time.Now()
			deliveryInfo := getDeliveryInfo(c.queueName, delivery)

			// Establish which handler is invoked
			handler, ok := c.options.handlers[deliveryInfo.RoutingKey]
			if !ok {
				if c.options.defaultHandler == nil {
					_ = delivery.Nack(false, false)
					continue
				}
				handler = c.options.defaultHandler
			}

			uevt := unmarshalEvent{DeliveryInfo: deliveryInfo}
			if err := json.Unmarshal(delivery.Body, &uevt); err != nil {
				_ = delivery.Nack(false, false)
				continue
			}

			tracingCtx := extractAMQPHeaders(delivery.Headers)
			if err := handler(tracingCtx, uevt); err != nil {
				notifyEventHandlerFailed(c.options.notificationCh, deliveryInfo.RoutingKey, err)
				_ = delivery.Nack(false, false)
				continue
			}

			elapsed := time.Since(startTime).Milliseconds()
			notifyEventHandlerSucceed(c.options.notificationCh, deliveryInfo.RoutingKey, elapsed)
			_ = delivery.Ack(false)
		}

		if !channel.IsClosed() {
			channel.Close()
		}

		notifyChannelLost(c.options.notificationCh, NotificationSourceConsumer)

		if err = c.Consume(); err != nil {
			notifyChannelFailed(c.options.notificationCh, NotificationSourceConsumer, err)
		}
	}()

	return nil
}

func getDeliveryInfo(queueName string, delivery amqp.Delivery) DeliveryInfo {
	deliveryInfo := DeliveryInfo{
		Queue:      queueName,
		Exchange:   delivery.Exchange,
		RoutingKey: delivery.RoutingKey,
	}

	// If routing key is empty, it is mostly due to the event being dead lettered.
	// Check for the original delivery information in the headers
	if delivery.RoutingKey == "" {
		deaths, ok := delivery.Headers["x-death"].([]interface{})
		if !ok || len(deaths) == 0 {
			return deliveryInfo
		}

		death, ok := deaths[0].(amqp.Table)
		if !ok {
			return deliveryInfo
		}

		queue, ok := death["queue"].(string)
		if !ok {
			return deliveryInfo
		}
		deliveryInfo.Queue = queue

		exchange, ok := death["exchange"].(string)
		if !ok {
			return deliveryInfo
		}
		deliveryInfo.Exchange = exchange

		routingKeys, ok := death["routing-keys"].([]interface{})
		if !ok || len(routingKeys) == 0 {
			return deliveryInfo
		}
		key, ok := routingKeys[0].(string)
		if !ok {
			return deliveryInfo
		}
		deliveryInfo.RoutingKey = key
	}

	return deliveryInfo
}

func (c Consumer) createExchanges(channel *amqp.Channel) error {
	errs := make([]error, 0)

	if c.options.exchange != "" {
		errs = append(errs, channel.ExchangeDeclare(
			c.options.exchange,
			"direct",
			true,  // durable
			false, // auto-deleted
			false, // internal
			false, // no-wait
			nil,   // args
		))
	}

	if c.options.deadLetterQueue != "" {
		errs = append(errs, channel.ExchangeDeclare(
			fmt.Sprintf("%s-exchange", c.options.deadLetterQueue),
			"direct",
			true,  // durable
			false, // auto-deleted
			false, // internal
			false, // no-wait
			nil,   // args
		))
	}

	return errors.Join(errs...)
}

func (c Consumer) createQueues(channel *amqp.Channel) error {
	errs := make([]error, 0)

	amqpTable := amqp.Table{}

	if c.options.quorumQueue {
		amqpTable["x-queue-type"] = "quorum"
	}

	if c.options.deadLetterQueue != "" {
		amqpTable = amqp.Table{
			"x-dead-letter-exchange":    fmt.Sprintf("%s-exchange", c.options.deadLetterQueue),
			"x-dead-letter-routing-key": "",
		}

		_, err := channel.QueueDeclare(
			c.options.deadLetterQueue,
			true,  // durable
			false, // auto-delete
			false, // exclusive
			false, // no-wait
			amqp.Table{},
		)
		errs = append(errs, err)
	}

	_, err := channel.QueueDeclare(
		c.queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		amqpTable,
	)
	errs = append(errs, err)

	return errors.Join(errs...)
}

func (c Consumer) queueBind(channel *amqp.Channel) error {
	errs := make([]error, 0)

	if c.options.exchange != "" {
		for routingKey := range c.options.handlers {
			errs = append(errs, channel.QueueBind(
				c.queueName,
				routingKey,
				c.options.exchange,
				false,
				nil,
			))
		}
	}

	if c.options.deadLetterQueue != "" {
		errs = append(errs, channel.QueueBind(
			c.options.deadLetterQueue,
			"",
			fmt.Sprintf("%s-exchange", c.options.deadLetterQueue),
			false,
			nil,
		))
	}

	return errors.Join(errs...)
}
