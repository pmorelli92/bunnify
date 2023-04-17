package bunnify

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	logger       Logger
	inUseChannel *amqp.Channel
	getChannel   func() (*amqp.Channel, bool)
}

func (c *Connection) NewPublisher() *Publisher {
	return &Publisher{
		logger:     c.options.logger,
		getChannel: c.getNewChannel,
	}
}

func (p *Publisher) Publish(
	ctx context.Context,
	exchange, routingKey string,
	event PublishableEvent) error {

	if p.inUseChannel == nil || p.inUseChannel.IsClosed() {
		channel, connectionClosed := p.getChannel()
		if connectionClosed {
			return fmt.Errorf("the connection is closed by system, no available channels")
		}
		p.inUseChannel = channel
	}

	b, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.inUseChannel.PublishWithContext(ctx, exchange, routingKey, true, false, amqp.Publishing{
		// Headers:         map[string]interface{}{},
		// ContentType:     "",
		ContentEncoding: "application/json",
		// DeliveryMode:    0,
		// Priority:        0,
		CorrelationId: event.CorrelationID,
		// ReplyTo:         "",
		// Expiration:      "",
		MessageId: event.ID,
		Timestamp: event.Timestamp,
		// Type:            "",
		// UserId:          "",
		// AppId:           "",
		Body: b,
	})
}
