package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pmorelli92/bunnify/bunnify"
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

	var eventFromDeadLetter bunnify.ConsumableEvent[json.RawMessage]
	defaultHandler := func(ctx context.Context, event bunnify.ConsumableEvent[json.RawMessage]) error {
		eventFromDeadLetter = event
		return nil
	}

	// Exercise
	connection := bunnify.NewConnection()
	connection.Start()

	consumer, err := connection.NewConsumer(
		queueName,
		bunnify.WithQoS(2, 0),
		bunnify.WithBindingToExchange(exchangeName),
		bunnify.WithHandler(routingKey, eventHandler),
		bunnify.WithDeadLetterQueue(deadLetterQueueName),
	)
	if err != nil {
		t.Fatal(err)
	}

	consumer.Consume()

	deadLetterConsumer, err := connection.NewConsumer(
		deadLetterQueueName,
		bunnify.WithDefaultHandler(defaultHandler),
	)
	if err != nil {
		t.Fatal(err)
	}

	deadLetterConsumer.Consume()

	err = connection.NewPublisher().Publish(
		context.TODO(),
		exchangeName,
		routingKey,
		publishedEvent)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	// Assert
	if publishedEvent.ID != eventFromDeadLetter.ID {
		t.Fatalf("expected event ID %s, got %s", publishedEvent.ID, eventFromDeadLetter.ID)
	}
	if publishedEvent.CorrelationID != eventFromDeadLetter.CorrelationID {
		t.Fatalf("expected correlation ID %s, got %s", publishedEvent.CorrelationID, eventFromDeadLetter.CorrelationID)
	}
	if !publishedEvent.Timestamp.Equal(eventFromDeadLetter.Timestamp) {
		t.Fatalf("expected timestamp %s, got %s", publishedEvent.Timestamp, eventFromDeadLetter.Timestamp)
	}

	var jsonData map[string]interface{}
	if err = json.Unmarshal(eventFromDeadLetter.Payload, &jsonData); err != nil {
		t.Fatal(err)
	}

	if publishedOrderCreated.ID != jsonData["id"].(string) {
		t.Fatalf("expected order created ID %s, got %s", publishedOrderCreated.ID, jsonData["id"].(string))
	}
	if exchangeName != eventFromDeadLetter.DeliveryInfo.Exchange {
		t.Fatalf("expected exchange %s, got %s", exchangeName, eventFromDeadLetter.DeliveryInfo.Exchange)
	}
	if queueName != eventFromDeadLetter.DeliveryInfo.Queue {
		t.Fatalf("expected queue %s, got %s", queueName, eventFromDeadLetter.DeliveryInfo.Queue)
	}
	if routingKey != eventFromDeadLetter.DeliveryInfo.RoutingKey {
		t.Fatalf("expected routing key %s, got %s", routingKey, eventFromDeadLetter.DeliveryInfo.RoutingKey)
	}
}
