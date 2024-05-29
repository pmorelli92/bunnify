package bunnify

import (
	"encoding/json"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (c Consumer) loop(channel *amqp.Channel, deliveries <-chan amqp.Delivery) {
	mutex := sync.Mutex{}
	for delivery := range deliveries {
		c.handle(delivery, &mutex)
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

func (c Consumer) parallelLoop(channel *amqp.Channel, deliveries <-chan amqp.Delivery) {
	mutex := sync.Mutex{}
	for delivery := range deliveries {
		go c.handle(delivery, &mutex)
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

func (c Consumer) handle(delivery amqp.Delivery, mutex *sync.Mutex) {
	startTime := time.Now()
	deliveryInfo := getDeliveryInfo(c.queueName, delivery)
	eventReceived(c.queueName, deliveryInfo.RoutingKey)

	// Establish which handler is invoked
	mutex.Lock()
	handler, ok := c.options.handlers[deliveryInfo.RoutingKey]
	mutex.Unlock()
	if !ok {
		if c.options.defaultHandler == nil {
			_ = delivery.Nack(false, false)
			eventWithoutHandler(c.queueName, deliveryInfo.RoutingKey)
			return
		}
		handler = c.options.defaultHandler
	}

	uevt := unmarshalEvent{DeliveryInfo: deliveryInfo}

	// For this error to happen an event not published by Bunnify is required
	if err := json.Unmarshal(delivery.Body, &uevt); err != nil {
		_ = delivery.Nack(false, false)
		eventNotParsable(c.queueName, deliveryInfo.RoutingKey)
		return
	}

	tracingCtx := extractToContext(delivery.Headers)
	if err := handler(tracingCtx, uevt); err != nil {
		elapsed := time.Since(startTime).Milliseconds()
		notifyEventHandlerFailed(c.options.notificationCh, deliveryInfo.RoutingKey, elapsed, err)
		_ = delivery.Nack(false, c.shouldRetry(delivery.Headers))
		eventNack(c.queueName, deliveryInfo.RoutingKey, elapsed)
		return
	}

	elapsed := time.Since(startTime).Milliseconds()
	notifyEventHandlerSucceed(c.options.notificationCh, deliveryInfo.RoutingKey, elapsed)
	_ = delivery.Ack(false)
	eventAck(c.queueName, deliveryInfo.RoutingKey, elapsed)
}

func (c Consumer) shouldRetry(headers amqp.Table) bool {
	if c.options.retries <= 0 {
		return false
	}

	// before the first retry, the delivery count is not present (so 0)
	retries, ok := headers["x-delivery-count"]
	if !ok {
		return true
	}

	r, _ := retries.(int64)
	return c.options.retries > int(r)
}
