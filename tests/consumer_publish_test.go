package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pmorelli92/bunnify/bunnify"
)

func TestConsumerPublisher(t *testing.T) {
	// Setup
	queueName := uuid.NewString()
	exchangeName := uuid.NewString()
	routingKey := "order.orderCreated"

	type orderCreated struct {
		ID string `json:"id"`
	}

	publishedOrderCreated := orderCreated{
		ID: uuid.NewString(),
	}
	publishedEvent := bunnify.NewPublishableEvent(publishedOrderCreated)

	var consumedEvent bunnify.ConsumableEvent[orderCreated]
	eventHandler := func(ctx context.Context, event bunnify.ConsumableEvent[orderCreated]) error {
		consumedEvent = event
		return nil
	}

	// Exercise
	connection := bunnify.NewConnection(
		bunnify.WithURI("amqp://localhost:5672"),
		bunnify.WithReconnectInterval(1*time.Second),
		bunnify.WithConnectionLogger(bunnify.NewSilentLogger()),
	)
	connection.Start()

	consumer, err := connection.NewConsumer(
		queueName,
		bunnify.WithQuorumQueue(),
		bunnify.WithBindingToExchange(exchangeName),
		bunnify.WithHandler(routingKey, eventHandler),
		bunnify.WithConsumerLogger(bunnify.NewSilentLogger()),
	)
	if err != nil {
		t.Fatal(err)
	}

	consumer.Consume()

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
