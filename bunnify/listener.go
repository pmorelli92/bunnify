package bunnify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueListener struct {
	exchange   string
	queueName  string
	handlers   map[string]handlerFor
	getChannel func() (*amqp.Channel, bool)
	logger     Logger
}

func (c *Connection) NewQueueListener(
	exchange string,
	queueName string,
	handlers ...handlerFor) QueueListener {

	handlerMap := make(map[string]handlerFor, len(handlers))
	for _, h := range handlers {
		handlerMap[h.routingKey] = h
	}

	return QueueListener{
		exchange:   exchange,
		queueName:  queueName,
		handlers:   handlerMap,
		logger:     c.options.logger,
		getChannel: c.newChannel,
	}
}

func exchange(channel *amqp.Channel, exchange string) error {
	return channel.ExchangeDeclare(
		exchange,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // args
	)
}

func queue(channel *amqp.Channel, queue string) error {
	_, err := channel.QueueDeclare(
		queue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		amqp.Table{
			// "x-dead-letter-exchange":    "",
			// "x-dead-letter-routing-key": b.consumer.deadLetterQueue,
		},
	)
	return err
}

func queueBind(channel *amqp.Channel, queue, exchange string, handlers map[string]handlerFor) error {
	errs := make([]error, 0)

	for routingKey := range handlers {
		if err := channel.QueueBind(
			queue,
			routingKey,
			exchange,
			false,
			nil,
		); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (b QueueListener) Listen() {
	channel, connectionClosed := b.getChannel()
	if connectionClosed {
		return
	}

	if err := exchange(channel, b.exchange); err != nil {
		b.logger.Error(fmt.Sprintf("failed to declare exchange: %s", err))
		return
	}

	if err := queue(channel, b.queueName); err != nil {
		b.logger.Error(fmt.Sprintf("failed to declare queue: %s", err))
		return
	}

	if err := queueBind(channel, b.queueName, b.exchange, b.handlers); err != nil {
		b.logger.Error(fmt.Sprintf("failed to bind queue: %s", err))
		return
	}

	deliveries, err := channel.Consume(b.queueName, "", false, false, false, false, nil)
	if err != nil {
		b.logger.Error(fmt.Sprintf("failed to establish consuming from queue: %s", err))
		return
	}

	go func() {
		for delivery := range deliveries {
			act, ok := b.handlers[delivery.RoutingKey]
			if !ok {
				_ = delivery.Nack(false, false)
				continue
			}

			var uevt unmarshalEvent
			if err := json.Unmarshal(delivery.Body, &uevt); err != nil {
				_ = delivery.Nack(false, false)
				continue
			}

			if err := act.handler(context.TODO(), uevt); err != nil {
				b.logger.Error(fmt.Sprintf("event handler for %s failed with: %s", delivery.RoutingKey, err.Error()))
				_ = delivery.Nack(false, false)
				continue
			}

			_ = delivery.Ack(false)
		}

		b.logger.Info(channelConnectionLost)
		go b.Listen()
	}()
}
