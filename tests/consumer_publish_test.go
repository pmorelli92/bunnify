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

func TestConsumerPublisher(t *testing.T) {
	// Setup
	queueName := uuid.NewString()
	exchangeName := uuid.NewString()
	routingKey := "order.orderCreated"

	type orderCreated struct {
		ID string `json:"id"`
	}

	exitCh := make(chan bool)
	notificationChannel := make(chan bunnify.Notification)
	go func() {
		for {
			select {
			case n := <-notificationChannel:
				fmt.Println(n)
			case <-exitCh:
				return
			}
		}
	}()

	// Exercise
	connection := bunnify.NewConnection(
		bunnify.WithURI("amqp://localhost:5672"),
		bunnify.WithReconnectInterval(1*time.Second),
		bunnify.WithNotificationChannel(notificationChannel))

	if err := connection.Start(); err != nil {
		t.Fatal(err)
	}

	var consumedEvent bunnify.ConsumableEvent[orderCreated]
	eventHandler := func(ctx context.Context, event bunnify.ConsumableEvent[orderCreated]) error {
		consumedEvent = event
		return nil
	}

	consumer := connection.NewConsumer(
		queueName,
		bunnify.WithQuorumQueue(),
		bunnify.WithBindingToExchange(exchangeName),
		bunnify.WithHandler(routingKey, eventHandler))

	if err := consumer.Consume(); err != nil {
		t.Fatal(err)
	}

	publisher := connection.NewPublisher()

	orderCreatedID := uuid.NewString()
	eventToPublish := bunnify.NewPublishableEvent(orderCreated{
		ID: orderCreatedID,
	})

	err := publisher.Publish(
		context.TODO(),
		exchangeName,
		routingKey,
		eventToPublish)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	// Stop the notification go routine so goleak does not fail
	exitCh <- true

	// Assert
	if eventToPublish.ID != consumedEvent.ID {
		t.Fatalf("expected event ID %s, got %s", eventToPublish.ID, consumedEvent.ID)
	}
	if eventToPublish.CorrelationID != consumedEvent.CorrelationID {
		t.Fatalf("expected correlation ID %s, got %s", eventToPublish.CorrelationID, consumedEvent.CorrelationID)
	}
	if !eventToPublish.Timestamp.Equal(consumedEvent.Timestamp) {
		t.Fatalf("expected timestamp %s, got %s", eventToPublish.Timestamp, consumedEvent.Timestamp)
	}
	if orderCreatedID != consumedEvent.Payload.ID {
		t.Fatalf("expected order created ID %s, got %s", orderCreatedID, consumedEvent.Payload.ID)
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

	goleak.VerifyNone(t)
}

func TestConsumerDefaultHandler(t *testing.T) {
	// Setup
	queueName := uuid.NewString()

	type orderCreated struct {
		ID string `json:"id"`
	}

	type orderUpdated struct {
		ID        string    `json:"id"`
		UpdatedAt time.Time `json:"updatedAt"`
	}

	connection := bunnify.NewConnection()
	if err := connection.Start(); err != nil {
		t.Fatal(err)
	}

	var consumedEvents []bunnify.ConsumableEvent[json.RawMessage]
	eventHandler := func(ctx context.Context, event bunnify.ConsumableEvent[json.RawMessage]) error {
		consumedEvents = append(consumedEvents, event)
		return nil
	}

	// Bind only to queue received messages
	consumer := connection.NewConsumer(
		queueName,
		bunnify.WithDefaultHandler(eventHandler))

	if err := consumer.Consume(); err != nil {
		t.Fatal(err)
	}

	orderCreatedEvent := orderCreated{ID: uuid.NewString()}
	orderUpdatedEvent := orderUpdated{ID: uuid.NewString(), UpdatedAt: time.Now()}
	publisher := connection.NewPublisher()

	// Publish directly to the queue, without routing key
	err := publisher.Publish(
		context.TODO(),
		"",
		queueName,
		bunnify.NewPublishableEvent(orderCreatedEvent))
	if err != nil {
		t.Fatal(err)
	}

	// Publish directly to the queue, without routing key
	err = publisher.Publish(
		context.TODO(),
		"",
		queueName,
		bunnify.NewPublishableEvent(orderUpdatedEvent))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	// Assert
	if len(consumedEvents) != 2 {
		t.Fatalf("expected 2 events, got %d", len(consumedEvents))
	}

	// First event should be orderCreated
	var receivedOrderCreated orderCreated
	err = json.Unmarshal(consumedEvents[0].Payload, &receivedOrderCreated)
	if err != nil {
		t.Fatal(err)
	}

	if orderCreatedEvent.ID != receivedOrderCreated.ID {
		t.Fatalf("expected created order ID to be %s got %s", orderCreatedEvent.ID, receivedOrderCreated.ID)
	}

	var receivedOrderUpdated orderUpdated
	err = json.Unmarshal(consumedEvents[1].Payload, &receivedOrderUpdated)
	if err != nil {
		t.Fatal(err)
	}

	if orderUpdatedEvent.ID != receivedOrderUpdated.ID {
		t.Fatalf("expected updated order ID to be %s got %s", orderUpdatedEvent.ID, receivedOrderUpdated.ID)
	}

	if !orderUpdatedEvent.UpdatedAt.Equal(receivedOrderUpdated.UpdatedAt) {
		t.Fatalf("expected updated order time to be %s got %s", orderUpdatedEvent.UpdatedAt, receivedOrderUpdated.UpdatedAt)
	}

	goleak.VerifyNone(t)
}
