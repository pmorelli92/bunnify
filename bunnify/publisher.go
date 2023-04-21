package bunnify

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher is used for publishing events.
type Publisher struct {
	inUseChannel  *amqp.Channel
	getNewChannel func() (*amqp.Channel, bool)
}

// NewPublisher creates a publisher using the specified connection.
func (c *Connection) NewPublisher() *Publisher {
	return &Publisher{
		getNewChannel: func() (*amqp.Channel, bool) {
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

	if p.inUseChannel == nil || p.inUseChannel.IsClosed() {
		channel, connectionClosed := p.getNewChannel()
		if connectionClosed {
			return fmt.Errorf("connection closed by system, channel will not reconnect")
		}
		p.inUseChannel = channel
	}

	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("could not marshal event: %w", err)
	}

	return p.inUseChannel.PublishWithContext(ctx, exchange, routingKey, true, false, amqp.Publishing{
		ContentEncoding: "application/json",
		CorrelationId:   event.CorrelationID,
		MessageId:       event.ID,
		Timestamp:       event.Timestamp,
		Body:            b,
	})
}
