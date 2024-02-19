package bunnify

import (
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

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

	go c.loop(channel, deliveries)
	return nil
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

	if c.options.quorumQueue && c.options.retries != 0 {
		amqpTable["x-delivery-count"] = c.options.retries
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
