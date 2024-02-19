package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pmorelli92/bunnify"
	"go.uber.org/goleak"
)

func TestConsumerRetriesShouldFailWhenNoQuorumQueues(t *testing.T) {
	// Setup
	queueName := uuid.NewString()
	exchangeName := uuid.NewString()

	// Exercise
	connection := bunnify.NewConnection()
	if err := connection.Start(); err != nil {
		t.Fatal(err)
	}

	consumer := connection.NewConsumer(
		queueName,
		bunnify.WithRetries(1),
		bunnify.WithBindingToExchange(exchangeName),
		bunnify.WithDefaultHandler(func(ctx context.Context, event bunnify.ConsumableEvent[json.RawMessage]) error {
			return nil
		}))

	err := consumer.Consume()
	if err == nil {
		t.Fatal("expected error as retry cannot be used without quorum queues")
	}

	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	goleak.VerifyNone(t)
}

func TestConsumerRetries(t *testing.T) {
	// Setup
	queueName := uuid.NewString()
	exchangeName := uuid.NewString()
	routingKey := "order.orderCreated"
	expectedRetries := 2

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

	actualProcessing := 0
	eventHandler := func(ctx context.Context, event bunnify.ConsumableEvent[orderCreated]) error {
		actualProcessing++
		return fmt.Errorf("error, this event should be retried")
	}

	// Exercise
	connection := bunnify.NewConnection()
	if err := connection.Start(); err != nil {
		t.Fatal(err)
	}

	consumer := connection.NewConsumer(
		queueName,
		bunnify.WithQuorumQueue(),
		bunnify.WithRetries(expectedRetries),
		bunnify.WithBindingToExchange(exchangeName),
		bunnify.WithHandler(routingKey, eventHandler))

	if err := consumer.Consume(); err != nil {
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
	expectedProcessing := expectedRetries + 1
	if expectedProcessing != actualProcessing {
		t.Fatalf("expected processing %d, got %d", expectedProcessing, actualProcessing)
	}

	goleak.VerifyNone(t)
}

func TestConsumerRetriesWithDeadLetterQueue(t *testing.T) {
	// Setup
	queueName := uuid.NewString()
	deadLetterQueueName := uuid.NewString()
	exchangeName := uuid.NewString()
	routingKey := "order.orderCreated"
	expectedRetries := 2

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

	actualProcessing := 0
	eventHandler := func(ctx context.Context, event bunnify.ConsumableEvent[orderCreated]) error {
		actualProcessing++
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
		bunnify.WithQuorumQueue(),
		bunnify.WithRetries(expectedRetries),
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
	expectedProcessing := expectedRetries + 1
	if expectedProcessing != actualProcessing {
		t.Fatalf("expected processing %d, got %d", expectedProcessing, actualProcessing)
	}

	if publishedEvent.ID != deadEvent.ID {
		t.Fatalf("expected event ID %s, got %s", publishedEvent.ID, deadEvent.ID)
	}

	goleak.VerifyNone(t)
}
