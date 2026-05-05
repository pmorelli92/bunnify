package tests

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pmorelli92/bunnify"
	"go.uber.org/goleak"
)

// TestPublisherConcurrentPublish verifies that a single Publisher can be used from multiple goroutines safely.
func TestPublisherConcurrentPublish(t *testing.T) {
	const goroutines = 16
	const perGoroutine = 25
	const expected = goroutines * perGoroutine

	queueName := uuid.NewString()
	exchangeName := uuid.NewString()
	routingKey := "order.orderCreated"

	type orderCreated struct {
		ID string `json:"id"`
	}

	connection := bunnify.NewConnection(
		bunnify.WithReconnectInterval(1 * time.Second))

	if err := connection.Start(); err != nil {
		t.Fatal(err)
	}

	var received atomic.Int64
	consumed := make(chan struct{}, expected)
	eventHandler := func(ctx context.Context, event bunnify.ConsumableEvent[orderCreated]) error {
		received.Add(1)
		consumed <- struct{}{}
		return nil
	}

	consumer := connection.NewConsumer(
		queueName,
		bunnify.WithBindingToExchange(exchangeName),
		bunnify.WithHandler(routingKey, eventHandler))

	if err := consumer.Consume(); err != nil {
		t.Fatal(err)
	}

	publisher := connection.NewPublisher()

	var wg sync.WaitGroup
	wg.Add(goroutines)
	errCh := make(chan error, goroutines*perGoroutine)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range perGoroutine {
				event := bunnify.NewPublishableEvent(orderCreated{ID: uuid.NewString()})
				if err := publisher.Publish(context.Background(), exchangeName, routingKey, event); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("publish failed: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for range expected {
		select {
		case <-consumed:
		case <-deadline:
			t.Fatalf("timed out waiting for events: got %d/%d", received.Load(), expected)
		}
	}

	if got := received.Load(); got != int64(expected) {
		t.Fatalf("expected %d events, got %d", expected, got)
	}

	if err := connection.Close(); err != nil {
		t.Fatal(fmt.Errorf("close: %w", err))
	}

	goleak.VerifyNone(t)
}
