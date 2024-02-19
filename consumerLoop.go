package bunnify

import (
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (c Consumer) loop(channel *amqp.Channel, deliveries <-chan amqp.Delivery) {
	for delivery := range deliveries {
		startTime := time.Now()
		deliveryInfo := getDeliveryInfo(c.queueName, delivery)
		EventReceived(c.queueName, deliveryInfo.RoutingKey)

		// Establish which handler is invoked
		handler, ok := c.options.handlers[deliveryInfo.RoutingKey]
		if !ok {
			if c.options.defaultHandler == nil {
				_ = delivery.Nack(false, false)
				EventWithoutHandler(c.queueName, deliveryInfo.RoutingKey)
				continue
			}
			handler = c.options.defaultHandler
		}

		uevt := unmarshalEvent{DeliveryInfo: deliveryInfo}

		// For this error to happen an event not published by Bunnify is required
		if err := json.Unmarshal(delivery.Body, &uevt); err != nil {
			_ = delivery.Nack(false, false)
			EventNotParsable(c.queueName, deliveryInfo.RoutingKey)
			continue
		}

		tracingCtx := extractToContext(delivery.Headers)
		if err := handler(tracingCtx, uevt); err != nil {
			elapsed := time.Since(startTime).Milliseconds()
			notifyEventHandlerFailed(c.options.notificationCh, deliveryInfo.RoutingKey, elapsed, err)
			_ = delivery.Nack(false, c.shouldRetry(delivery.Headers))
			EventNack(c.queueName, deliveryInfo.RoutingKey, elapsed)
			continue
		}

		elapsed := time.Since(startTime).Milliseconds()
		notifyEventHandlerSucceed(c.options.notificationCh, deliveryInfo.RoutingKey, elapsed)
		_ = delivery.Ack(false)
		EventAck(c.queueName, deliveryInfo.RoutingKey, elapsed)
	}

	// If the for exits, the channel stopped. Close it,
	// notify the error and start the consumer so it will start another loop.
	if !channel.IsClosed() {
		channel.Close()
	}

	notifyChannelLost(c.options.notificationCh, NotificationSourceConsumer)

	if err := c.Consume(); err != nil {
		notifyChannelFailed(c.options.notificationCh, NotificationSourceConsumer, err)
	}
}

func (c Consumer) shouldRetry(headers amqp.Table) bool {
	if !(c.options.retries > 0) {
		return false
	}

	// before the first retry, the delivery count is not present (so 0)
	retries, ok := headers["x-delivery-count"]
	if !ok {
		return true
	}

	r, ok := retries.(int64)
	if !ok {
		return false
	}

	return c.options.retries > int(r)
}
