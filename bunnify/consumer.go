package bunnify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type consumerOption struct {
	logger          Logger
	deadLetterQueue string
	exchange        string
	defaultHandler  wrappedHandler
	handlers        map[string]wrappedHandler
}

// WithConsumerLogger specifies which logger to use for channel
// related messages such as connection established, reconnecting, etc.
func WithConsumerLogger(logger Logger) func(*consumerOption) {
	return func(opt *consumerOption) {
		opt.logger = logger
	}
}

// WithBindingToExchange specifies the exchange on which the queue
// will bind for the handlers provided.
func WithBindingToExchange(exchange string) func(*consumerOption) {
	return func(opt *consumerOption) {
		opt.exchange = exchange
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
	queueName  string
	options    consumerOption
	getChannel func() (*amqp.Channel, bool)
}

// NewConsumer creates a consumer for a given queue using the specified connection.
// If handlers or default handler are not specified, it will return error.
func (c *Connection) NewConsumer(
	queueName string,
	opts ...func(*consumerOption)) (Consumer, error) {

	options := consumerOption{
		logger:   NewDefaultLogger(),
		handlers: make(map[string]wrappedHandler, 0),
	}
	for _, opt := range opts {
		opt(&options)
	}

	if options.defaultHandler == nil && len(options.handlers) == 0 {
		return Consumer{}, fmt.Errorf("no handlers specified")
	}

	return Consumer{
		queueName:  queueName,
		options:    options,
		getChannel: c.getNewChannel,
	}, nil
}

// Consume will start consuming events for the indicated queue.
func (c Consumer) Consume() {
	channel, connectionClosed := c.getChannel()
	if connectionClosed {
		return
	}

	if err := c.createExchanges(channel); err != nil {
		c.options.logger.Error(fmt.Sprintf("failed to declare exchange: %s", err))
		return
	}

	if err := c.createQueues(channel); err != nil {
		c.options.logger.Error(fmt.Sprintf("failed to declare queue: %s", err))
		return
	}

	if err := c.queueBind(channel); err != nil {
		c.options.logger.Error(fmt.Sprintf("failed to bind queue: %s", err))
		return
	}

	deliveries, err := channel.Consume(c.queueName, "", false, false, false, false, nil)
	if err != nil {
		c.options.logger.Error(fmt.Sprintf("failed to establish consuming from queue: %s", err))
		return
	}

	go func() {
		for delivery := range deliveries {
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
				fmt.Println(err)
				_ = delivery.Nack(false, false)
				continue
			}

			if err := handler(context.TODO(), uevt); err != nil {
				c.options.logger.Error(fmt.Sprintf("event handler for %s failed with: %s", delivery.RoutingKey, err.Error()))
				_ = delivery.Nack(false, false)
				continue
			}

			_ = delivery.Ack(false)
		}

		c.options.logger.Info(channelConnectionLost)
		go c.Consume()
	}()
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
