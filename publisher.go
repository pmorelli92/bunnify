package bunnify

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher is used for publishing events.
type Publisher struct {
	getNewChannel func() (*amqp.Channel, error)
}

// NewPublisher creates a publisher using the specified connection.
func (c *Connection) NewPublisher() *Publisher {
	return &Publisher{
		getNewChannel: func() (*amqp.Channel, error) {
			return c.getNewChannel(NotificationSourcePublisher)
		},
	}
}

// Publish publishes an event to the specified exchange.
// If the channel is closed, it will retry until a channel is obtained.
func (p *Publisher) Publish(
	ctx context.Context,
	exchange, routingKey string,
	event PublishableEvent) error {

	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("could not marshal event: %w", err)
	}

	publishing := amqp.Publishing{
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		CorrelationId: event.CorrelationID,
		MessageId:       event.ID,
		Timestamp:       event.Timestamp,
		Body:            b,
		Headers:         injectToHeaders(ctx),
	}

	ch, err := p.getNewChannel()
	if err != nil {
		return fmt.Errorf("connection closed by system, channel will not reconnect: %w", err)
	}
	defer ch.Close()

	// Detect mid-publish channel death and surface it to the caller
	closeCh := ch.NotifyClose(make(chan *amqp.Error, 1))
	publishCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-closeCh:
			cancel()
		case <-publishCtx.Done():
		}
	}()

	err = ch.PublishWithContext(publishCtx, exchange, routingKey, true, false, publishing)
	if err != nil {
		eventPublishFailed(exchange, routingKey)
		return err
	}

	eventPublishSucceed(exchange, routingKey)
	return nil
}
