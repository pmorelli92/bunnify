package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pmorelli92/bunnify/bunnify"
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

	connection.Start()

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
