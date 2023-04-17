package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pmorelli92/bunnify/bunnify"
)

func TestConsumerPublisher(t *testing.T) {
	// Setup
	queueName := fmt.Sprintf("t1-queue-%d", time.Now().Unix())
	exchangeName := fmt.Sprintf("t1-exchange-%d", time.Now().Unix())
	routingKey := "order.orderCreated"

	type orderCreated struct {
		ID string `json:"id"`
	}

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

	c.NewListener(
		queueName,
		bunnify.WithBindingTo(exchangeName),
		bunnify.WithHandler(routingKey, eventHandler),
	).Listen()

	err := c.NewPublisher().Publish(
		context.TODO(),
		exchangeName,
		routingKey,
		publishedEvent)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	// Assert
	if publishedEvent.ID != consumedEvent.ID {
		t.Fatalf("expected event ID %s, got %s", publishedEvent.ID, consumedEvent.ID)
	}
	if publishedEvent.CorrelationID != consumedEvent.CorrelationID {
		t.Fatalf("expected correlation ID %s, got %s", publishedEvent.CorrelationID, consumedEvent.CorrelationID)
	}
	if !publishedEvent.Timestamp.Equal(consumedEvent.Timestamp) {
		t.Fatalf("expected timestamp %s, got %s", publishedEvent.Timestamp, consumedEvent.Timestamp)
	}
	if publishedOrderCreated.ID != consumedEvent.Payload.ID {
		t.Fatalf("expected order created ID %s, got %s", publishedOrderCreated.ID, consumedEvent.Payload.ID)
	}
	if exchangeName != consumedEvent.DeliveryInfo.Exchange {
		t.Fatalf("expected exchange %s, got %s", exchangeName, consumedEvent.DeliveryInfo.Exchange)
	}
	if queueName != consumedEvent.DeliveryInfo.Queue {
		t.Fatalf("expected queue %s, got %s", queueName, consumedEvent.DeliveryInfo.Queue)
	}
	if routingKey != consumedEvent.DeliveryInfo.RoutingKey {
		t.Fatalf("expected routing key %s, got %s", routingKey, consumedEvent.DeliveryInfo.RoutingKey)
	}
}
