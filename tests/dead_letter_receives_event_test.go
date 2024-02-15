package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pmorelli92/bunnify"
	"go.uber.org/goleak"
)

func TestDeadLetterReceivesEvent(t *testing.T) {
	// Setup
	queueName := uuid.NewString()
	deadLetterQueueName := uuid.NewString()
	exchangeName := uuid.NewString()
	routingKey := "order.orderCreated"

	type orderCreated struct {
		ID string `json:"id"`
	}

	publishedOrderCreated := orderCreated{
		ID: uuid.NewString(),
	}
	publishedEvent := bunnify.NewPublishableEvent(
		publishedOrderCreated,
		bunnify.WithEventID("custom-event-id"),
		bunnify.WithCorrelationID("custom-correlation-id"),
	)

	eventHandler := func(ctx context.Context, event bunnify.ConsumableEvent[orderCreated]) error {
		return fmt.Errorf("error, this event will go to dead-letter")
	}

	var deadEvent bunnify.ConsumableEvent[orderCreated]
	deadEventHandler := func(ctx context.Context, event bunnify.ConsumableEvent[orderCreated]) error {
		deadEvent = event
		return nil
	}

	// Exercise
	connection := bunnify.NewConnection()
	if err := connection.Start(); err != nil {
		t.Fatal(err)
	}

	consumer := connection.NewConsumer(
		queueName,
		bunnify.WithQoS(2, 0),
		bunnify.WithBindingToExchange(exchangeName),
		bunnify.WithHandler(routingKey, eventHandler),
		bunnify.WithDeadLetterQueue(deadLetterQueueName))

	if err := consumer.Consume(); err != nil {
		t.Fatal(err)
	}

	deadLetterConsumer := connection.NewConsumer(
		deadLetterQueueName,
		bunnify.WithHandler(routingKey, deadEventHandler))

	if err := deadLetterConsumer.Consume(); err != nil {
		t.Fatal(err)
	}

	publisher := connection.NewPublisher()

	err := publisher.Publish(context.TODO(), exchangeName, routingKey, publishedEvent)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	// Assert
	if publishedEvent.ID != deadEvent.ID {
		t.Fatalf("expected event ID %s, got %s", publishedEvent.ID, deadEvent.ID)
	}
	if publishedEvent.CorrelationID != deadEvent.CorrelationID {
		t.Fatalf("expected correlation ID %s, got %s", publishedEvent.CorrelationID, deadEvent.CorrelationID)
	}
	if !publishedEvent.Timestamp.Equal(deadEvent.Timestamp) {
		t.Fatalf("expected timestamp %s, got %s", publishedEvent.Timestamp, deadEvent.Timestamp)
	}

	if publishedOrderCreated.ID != deadEvent.Payload.ID {
		t.Fatalf("expected order created ID %s, got %s", publishedOrderCreated.ID, deadEvent.Payload.ID)
	}
	if exchangeName != deadEvent.DeliveryInfo.Exchange {
		t.Fatalf("expected exchange %s, got %s", exchangeName, deadEvent.DeliveryInfo.Exchange)
	}
	if queueName != deadEvent.DeliveryInfo.Queue {
		t.Fatalf("expected queue %s, got %s", queueName, deadEvent.DeliveryInfo.Queue)
	}
	if routingKey != deadEvent.DeliveryInfo.RoutingKey {
		t.Fatalf("expected routing key %s, got %s", routingKey, deadEvent.DeliveryInfo.RoutingKey)
	}

	goleak.VerifyNone(t)
}
