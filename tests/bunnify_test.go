package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pmorelli92/bunnify/bunnify"
)

func TestSingleConsumerPublisher(t *testing.T) {
	// Setup
	queueName := fmt.Sprintf("queue-%d", time.Now().Unix())
	exchangeName := fmt.Sprintf("exchange-%d", time.Now().Unix())
	routingKey := "order.orderCreated"

	publishedOrderCreated := orderCreated{
		ID: fmt.Sprint(time.Now().Unix()),
	}
	publishedEvent := bunnify.PublishableEvent{
		Metadata: bunnify.Metadata{
			ID:            fmt.Sprint(time.Now().Unix()),
			CorrelationID: fmt.Sprint(time.Now().Unix()),
			Timestamp:     time.Now(),
		},
		Payload: publishedOrderCreated,
	}

	var consumedEvent bunnify.ConsumableEvent[orderCreated]
	eventHandler := func(ctx context.Context, event bunnify.ConsumableEvent[orderCreated]) error {
		consumedEvent = event
		return nil
	}

	// Exercise
	c := bunnify.NewConnection()
	c.Start()

	c.NewQueueListener(
		exchangeName,
		queueName,
		bunnify.NewHandlerFor(routingKey, eventHandler),
	).Listen()

	if err := c.NewPublisher().Publish(
		context.TODO(),
		exchangeName,
		routingKey,
		publishedEvent); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	// Assert
	if publishedEvent.ID != consumedEvent.ID {
		t.Fatal("ous")
	}
	if publishedEvent.CorrelationID != consumedEvent.CorrelationID {
		t.Fatal("ous")
	}
	if !publishedEvent.Timestamp.Equal(consumedEvent.Timestamp) {
		why := fmt.Sprintf("p: %s and c: %s", publishedEvent.Timestamp, consumedEvent.Timestamp)
		t.Fatal(why)
	}
	if publishedOrderCreated.ID != consumedEvent.Payload.ID {
		t.Fatal("ous")
	}
}

type orderCreated struct {
	ID string `json:"id"`
}
