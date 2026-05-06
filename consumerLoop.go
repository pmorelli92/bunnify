package bunnify

import (
	"encoding/json"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (c *Consumer) loop(channel *amqp.Channel, deliveries <-chan amqp.Delivery) {
	closeCh := channel.NotifyClose(make(chan *amqp.Error, 1))
	go func() {
		if _, ok := <-closeCh; ok {
			channel.Close()
		}
	}()

	for delivery := range deliveries {
		c.handle(delivery)
	}

	// If the for exits, it means the channel stopped. Close it and try to reconnect
	if !channel.IsClosed() {
		channel.Close()
	}

	c.initialized = false
	err := c.Consume()
	// c.Consume() calls c.wg.Add(1) before launching the successor goroutine,
	// so Done() here pairs with the Add(1) that launched this goroutine.
	defer c.wg.Done()
	if isTerminalConnError(err) {
		return
	}

	notifyChannelLost(c.options.notificationCh, NotificationSourceConsumer)
	if err != nil {
		notifyChannelFailed(c.options.notificationCh, NotificationSourceConsumer, err)
	}
}

func (c *Consumer) parallelLoop(channel *amqp.Channel, deliveries <-chan amqp.Delivery) {
	closeCh := channel.NotifyClose(make(chan *amqp.Error, 1))
	go func() {
		if _, ok := <-closeCh; ok {
			channel.Close()
		}
	}()

	var wg sync.WaitGroup
	sem := make(chan struct{}, c.options.maxParallelHandlers)
	for delivery := range deliveries {
		if c.options.maxParallelHandlers > 0 {
			sem <- struct{}{} // acquire
		}
		wg.Add(1)
		go func(d amqp.Delivery) {
			defer wg.Done()
			if c.options.maxParallelHandlers > 0 {
				defer func() { <-sem }() // release
			}
			c.handle(d)
		}(delivery)
	}
	wg.Wait()

	if !channel.IsClosed() {
		channel.Close()
	}

	c.initialized = false
	err := c.ConsumeParallel()
	// c.ConsumeParallel() calls c.wg.Add(1) before launching the successor goroutine,
	// so Done() here pairs with the Add(1) that launched this goroutine.
	defer c.wg.Done()
	if isTerminalConnError(err) {
		return
	}

	notifyChannelLost(c.options.notificationCh, NotificationSourceConsumer)
	if err != nil {
		notifyChannelFailed(c.options.notificationCh, NotificationSourceConsumer, err)
	}
}

func (c *Consumer) handle(delivery amqp.Delivery) {
	startTime := time.Now()
	deliveryInfo := getDeliveryInfo(c.queueName, delivery)
	eventReceived(c.queueName, deliveryInfo.RoutingKey)

	c.options.mu.RLock()
	handler, ok := c.options.handlers[deliveryInfo.RoutingKey]
	c.options.mu.RUnlock()
	if !ok {
		if c.options.defaultHandler == nil {
			if err := delivery.Nack(false, false); err != nil {
				notifyChannelFailed(c.options.notificationCh, NotificationSourceConsumer, err)
			}
			notifyEventHandlerNotFound(c.options.notificationCh, deliveryInfo.RoutingKey)
			eventWithoutHandler(c.queueName, deliveryInfo.RoutingKey)
			return
		}
		handler = c.options.defaultHandler
	}

	uevt := unmarshalEvent{DeliveryInfo: deliveryInfo}

	// For this error to happen an event not published by Bunnify is required
	if err := json.Unmarshal(delivery.Body, &uevt); err != nil {
		if nackErr := delivery.Nack(false, false); nackErr != nil {
			notifyChannelFailed(c.options.notificationCh, NotificationSourceConsumer, nackErr)
		}
		eventNotParsable(c.queueName, deliveryInfo.RoutingKey)
		return
	}

	tracingCtx := extractToContext(delivery.Headers)
	if err := handler(tracingCtx, uevt); err != nil {
		elapsed := time.Since(startTime).Milliseconds()
		notifyEventHandlerFailed(c.options.notificationCh, deliveryInfo.RoutingKey, elapsed, err)
		if nackErr := delivery.Nack(false, c.shouldRetry(delivery.Headers)); nackErr != nil {
			notifyChannelFailed(c.options.notificationCh, NotificationSourceConsumer, nackErr)
		}
		eventNack(c.queueName, deliveryInfo.RoutingKey, elapsed)
		return
	}

	elapsed := time.Since(startTime).Milliseconds()
	if err := delivery.Ack(false); err != nil {
		notifyChannelFailed(c.options.notificationCh, NotificationSourceConsumer, err)
		return
	}
	notifyEventHandlerSucceed(c.options.notificationCh, deliveryInfo.RoutingKey, elapsed)
	eventAck(c.queueName, deliveryInfo.RoutingKey, elapsed)
}

func (c *Consumer) shouldRetry(headers amqp.Table) bool {
	if c.options.retries <= 0 {
		return false
	}

	// On RabbitMQ 4+, basic.nack with requeue increments x-acquired-count on the
	// redelivery. On RabbitMQ 3, the equivalent counter was x-delivery-count.
	// Before the first retry, neither header is present (so 0).
	count, ok := headers["x-acquired-count"]
	if !ok {
		count, ok = headers["x-delivery-count"]
	}

	var r int64
	if ok {
		switch v := count.(type) {
		case int64:
			r = v
		case int32:
			r = int64(v)
		case int:
			r = int64(v)
		}
	}
	return c.options.retries > int(r)
}
