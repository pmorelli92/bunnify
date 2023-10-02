package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pmorelli92/bunnify/bunnify"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/goleak"
)

func TestConsumerPublisherMetrics(t *testing.T) {
	t.Run("ACK event", func(t *testing.T) {
		connection := bunnify.NewConnection()
		connection.Start()
		publisher := connection.NewPublisher()

		queueName := uuid.NewString()
		exchangeName := uuid.NewString()
		routingKey := uuid.NewString()

		r := prometheus.NewRegistry()
		err := bunnify.InitMetrics(r)
		if err != nil {
			t.Fatal(err)
		}

		// Multiple invocations should not fail
		err = bunnify.InitMetrics(r)
		if err != nil {
			t.Fatal()
		}

		// Exercise consuming
		eventHandler := func(ctx context.Context, _ bunnify.ConsumableEvent[any]) error {
			return nil
		}

		consumer := connection.NewConsumer(
			queueName,
			bunnify.WithBindingToExchange(exchangeName),
			bunnify.WithHandler(routingKey, eventHandler))
		if err := consumer.Consume(); err != nil {
			t.Fatal(err)
		}

		err = publisher.Publish(
			context.Background(),
			exchangeName,
			routingKey,
			bunnify.NewPublishableEvent(struct{}{}))
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(50 * time.Millisecond)

		if err := connection.Close(); err != nil {
			t.Fatal(err)
		}

		if err = assertMetrics(r,
			"amqp_events_publish_succeed",
			"amqp_events_received",
			"amqp_events_ack",
			"amqp_events_processed_duration"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("NACK event", func(t *testing.T) {
		connection := bunnify.NewConnection()
		connection.Start()
		publisher := connection.NewPublisher()

		queueName := uuid.NewString()
		exchangeName := uuid.NewString()
		routingKey := uuid.NewString()

		r := prometheus.NewRegistry()
		err := bunnify.InitMetrics(r)
		if err != nil {
			t.Fatal(err)
		}

		// Exercise consuming
		eventHandler := func(ctx context.Context, _ bunnify.ConsumableEvent[any]) error {
			return fmt.Errorf("error here")
		}

		consumer := connection.NewConsumer(
			queueName,
			bunnify.WithBindingToExchange(exchangeName),
			bunnify.WithHandler(routingKey, eventHandler))
		if err := consumer.Consume(); err != nil {
			t.Fatal(err)
		}

		err = publisher.Publish(
			context.Background(),
			exchangeName,
			routingKey,
			bunnify.NewPublishableEvent(struct{}{}))
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(50 * time.Millisecond)

		if err := connection.Close(); err != nil {
			t.Fatal(err)
		}

		if err = assertMetrics(r,
			"amqp_events_publish_succeed",
			"amqp_events_received",
			"amqp_events_nack",
			"amqp_events_processed_duration"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("No handler for the event", func(t *testing.T) {
		r := prometheus.NewRegistry()
		err := bunnify.InitMetrics(r)
		if err != nil {
			t.Fatal(err)
		}

		connection := bunnify.NewConnection()
		connection.Start()

		queueName := uuid.NewString()
		exchangeName := uuid.NewString()
		routingKey := uuid.NewString()

		// Exercise consuming
		eventHandler := func(ctx context.Context, _ bunnify.ConsumableEvent[any]) error {
			return nil
		}

		consumer := connection.NewConsumer(
			queueName,
			bunnify.WithBindingToExchange(exchangeName),
			bunnify.WithHandler(routingKey, eventHandler))
		if err := consumer.Consume(); err != nil {
			t.Fatal(err)
		}

		if err := connection.Close(); err != nil {
			t.Fatal(err)
		}

		connection = bunnify.NewConnection()
		connection.Start()

		// Register again but with other routing key
		// The existing binding on the AMQP instance still exists
		consumer = connection.NewConsumer(
			queueName,
			bunnify.WithBindingToExchange(exchangeName),
			bunnify.WithHandler("not-used-key", eventHandler))
		if err := consumer.Consume(); err != nil {
			t.Fatal(err)
		}

		publisher := connection.NewPublisher()

		err = publisher.Publish(
			context.Background(),
			exchangeName,
			routingKey,
			bunnify.NewPublishableEvent(struct{}{}))
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(50 * time.Millisecond)

		if err := connection.Close(); err != nil {
			t.Fatal(err)
		}

		if err = assertMetrics(r,
			"amqp_events_publish_succeed",
			"amqp_events_received",
			"amqp_events_without_handler"); err != nil {
			t.Fatal(err)
		}
	})

	goleak.VerifyNone(t)
}

func assertMetrics(
	prometheusGatherer prometheus.Gatherer,
	metrics ...string) error {

	errs := make([]error, 0)
	for _, m := range metrics {
		actualQuantity, err := testutil.GatherAndCount(prometheusGatherer, m)
		if err != nil {
			return err
		}

		if actualQuantity != 1 {
			errs = append(errs, fmt.Errorf("expected %s quantity 1, received %d", m, actualQuantity))
		}
	}

	return errors.Join(errs...)
}
