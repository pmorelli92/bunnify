package bunnify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type listenerOption struct {
	logger          Logger
	deadLetterQueue string
	exchange        string
	defaultHandler  wrappedHandler
	handlers        map[string]wrappedHandler
}

func WithListenerLogger(logger Logger) func(*listenerOption) {
	return func(opt *listenerOption) {
		opt.logger = logger
	}
}

func WithBindingTo(exchange string) func(*listenerOption) {
	return func(opt *listenerOption) {
		opt.exchange = exchange
	}
}

func WithDeadLetterQueue(queueName string) func(*listenerOption) {
	return func(opt *listenerOption) {
		opt.deadLetterQueue = queueName
	}
}

func WithDefaultHandler(handler EventHandler[json.RawMessage]) func(*listenerOption) {
	return func(opt *listenerOption) {
		opt.defaultHandler = newWrappedHandler(handler)
	}
}

func WithHandler[T any](routingKey string, handler EventHandler[T]) func(*listenerOption) {
	return func(opt *listenerOption) {
		opt.handlers[routingKey] = newWrappedHandler(handler)
	}
}

type Listener struct {
	queueName  string
	options    listenerOption
	getChannel func() (*amqp.Channel, bool)
}

func (c *Connection) NewListener(
	queueName string,
	opts ...func(*listenerOption)) Listener {

	options := listenerOption{
		logger:   NewDefaultLogger(),
		handlers: make(map[string]wrappedHandler, 0),
	}
	for _, opt := range opts {
		opt(&options)
	}

	return Listener{
		queueName:  queueName,
		options:    options,
		getChannel: c.getNewChannel,
	}
}

func (l Listener) Listen() {
	channel, connectionClosed := l.getChannel()
	if connectionClosed {
		return
	}

	if err := l.createExchanges(channel); err != nil {
		l.options.logger.Error(fmt.Sprintf("failed to declare exchange: %s", err))
		return
	}

	if err := l.createQueues(channel); err != nil {
		l.options.logger.Error(fmt.Sprintf("failed to declare queue: %s", err))
		return
	}

	if err := l.queueBind(channel); err != nil {
		l.options.logger.Error(fmt.Sprintf("failed to bind queue: %s", err))
		return
	}

	deliveries, err := channel.Consume(l.queueName, "", false, false, false, false, nil)
	if err != nil {
		l.options.logger.Error(fmt.Sprintf("failed to establish consuming from queue: %s", err))
		return
	}

	go func() {
		for delivery := range deliveries {
			deliveryInfo := getDeliveryInfo(l.queueName, delivery)

			// Establish which handler is invoked
			handler, ok := l.options.handlers[deliveryInfo.RoutingKey]
			if !ok {
				if l.options.defaultHandler == nil {
					_ = delivery.Nack(false, false)
					continue
				}
				handler = l.options.defaultHandler
			}

			uevt := unmarshalEvent{DeliveryInfo: deliveryInfo}
			if err := json.Unmarshal(delivery.Body, &uevt); err != nil {
				fmt.Println(err)
				_ = delivery.Nack(false, false)
				continue
			}

			if err := handler(context.TODO(), uevt); err != nil {
				l.options.logger.Error(fmt.Sprintf("event handler for %s failed with: %s", delivery.RoutingKey, err.Error()))
				_ = delivery.Nack(false, false)
				continue
			}

			_ = delivery.Ack(false)
		}

		l.options.logger.Info(channelConnectionLost)
		go l.Listen()
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

func (l Listener) createExchanges(channel *amqp.Channel) error {
	errs := make([]error, 0)

	if l.options.exchange != "" {
		errs = append(errs, channel.ExchangeDeclare(
			l.options.exchange,
			"direct",
			true,  // durable
			false, // auto-deleted
			false, // internal
			false, // no-wait
			nil,   // args
		))
	}

	if l.options.deadLetterQueue != "" {
		errs = append(errs, channel.ExchangeDeclare(
			fmt.Sprintf("%s-exchange", l.options.deadLetterQueue),
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

func (l Listener) createQueues(channel *amqp.Channel) error {
	errs := make([]error, 0)

	amqpTable := amqp.Table{}

	if l.options.deadLetterQueue != "" {
		amqpTable = amqp.Table{
			"x-dead-letter-exchange":    fmt.Sprintf("%s-exchange", l.options.deadLetterQueue),
			"x-dead-letter-routing-key": "",
		}

		_, err := channel.QueueDeclare(
			l.options.deadLetterQueue,
			true,  // durable
			false, // auto-delete
			false, // exclusive
			false, // no-wait
			amqp.Table{},
		)
		errs = append(errs, err)
	}

	_, err := channel.QueueDeclare(
		l.queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		amqpTable,
	)
	errs = append(errs, err)

	return errors.Join(errs...)
}

func (l Listener) queueBind(channel *amqp.Channel) error {
	errs := make([]error, 0)

	if l.options.exchange != "" {
		for routingKey := range l.options.handlers {
			errs = append(errs, channel.QueueBind(
				l.queueName,
				routingKey,
				l.options.exchange,
				false,
				nil,
			))
		}
	}

	if l.options.deadLetterQueue != "" {
		errs = append(errs, channel.QueueBind(
			l.options.deadLetterQueue,
			"",
			fmt.Sprintf("%s-exchange", l.options.deadLetterQueue),
			false,
			nil,
		))
	}

	return errors.Join(errs...)
}
