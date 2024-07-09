package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pmorelli92/bunnify"
	"github.com/pmorelli92/bunnify/outbox"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/goleak"
)

func TestOutboxWithAllAddsOn(t *testing.T) {
	// Setup tracing
	otel.SetTracerProvider(tracesdk.NewTracerProvider())
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Setup notification channel
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

	// Setup prometheus
	r := prometheus.NewRegistry()
	err := bunnify.InitMetrics(r)
	if err != nil {
		t.Fatal(err)
	}

	// Setup connection
	connection := bunnify.NewConnection(bunnify.WithNotificationChannel(notificationChannel))
	if err := connection.Start(); err != nil {
		t.Fatal(err)
	}

	// Setup consumer parameters
	queueName := uuid.NewString()
	exchangeName := uuid.NewString()
	routingKey := "order.orderCreated"

	type orderCreated struct {
		ID string `json:"id"`
	}

	// Populate variables in the closure when the consumer handler
	// is executed after event is published and the consumer is triggered
	var actualSpanID trace.SpanID
	var actualTraceID trace.TraceID
	var consumedEvent bunnify.ConsumableEvent[orderCreated]
	eventHandler := func(ctx context.Context, event bunnify.ConsumableEvent[orderCreated]) error {
		consumedEvent = event
		actualSpanID = trace.SpanFromContext(ctx).SpanContext().SpanID()
		actualTraceID = trace.SpanFromContext(ctx).SpanContext().TraceID()
		return nil
	}

	// Setup consumer
	consumer := connection.NewConsumer(
		queueName,
		bunnify.WithQuorumQueue(),
		bunnify.WithBindingToExchange(exchangeName),
		bunnify.WithHandler(routingKey, eventHandler))

	if err := consumer.Consume(); err != nil {
		t.Fatal(err)
	}

	// Setup publisher
	publisher := connection.NewPublisher()

	// Setup database connection
	dbCtx := context.TODO()
	db, err := pgxpool.New(dbCtx, "postgresql://db:pass@localhost:5432/db")
	if err != nil {
		t.Fatal(err)
	}

	// Setup outbox publisher
	outboxPublisher, err := outbox.NewPublisher(
		dbCtx, db, *publisher,
		outbox.WithLoopingInterval(500*time.Millisecond),
		outbox.WithNoficationChannel(notificationChannel))
	if err != nil {
		t.Fatal(err)
	}

	// Exercise
	orderCreatedID := uuid.NewString()
	eventToPublish := bunnify.NewPublishableEvent(orderCreated{ID: orderCreatedID})
	publisherCtx, _ := otel.Tracer("amqp").Start(context.Background(), "outbox-publisher")

	tx, err := db.Begin(dbCtx)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = tx.Rollback(dbCtx)
	}()

	err = outboxPublisher.Publish(publisherCtx, tx, exchangeName, routingKey, eventToPublish)
	if err != nil {
		t.Fatal(err)
	}

	if err := tx.Commit(dbCtx); err != nil {
		t.Fatal(err)
	}

	// Wait for the outbox loop to execute
	time.Sleep(1000 * time.Millisecond)

	// Assert tracing data
	expectedSpanID := trace.SpanFromContext(publisherCtx).SpanContext().SpanID()
	if actualSpanID != expectedSpanID {
		t.Fatalf("expected spanID %s, got %s", expectedSpanID, actualSpanID)
	}
	expectedTraceID := trace.SpanFromContext(publisherCtx).SpanContext().TraceID()
	if actualTraceID != expectedTraceID {
		t.Fatalf("expected traceID %s, got %s", expectedTraceID, actualTraceID)
	}

	// Assert event data
	if orderCreatedID != consumedEvent.Payload.ID {
		t.Fatalf("expected order created ID %s, got %s", orderCreatedID, consumedEvent.Payload.ID)
	}

	// Assert event metadata
	if eventToPublish.ID != consumedEvent.ID {
		t.Fatalf("expected event ID %s, got %s", eventToPublish.ID, consumedEvent.ID)
	}
	if eventToPublish.CorrelationID != consumedEvent.CorrelationID {
		t.Fatalf("expected correlation ID %s, got %s", eventToPublish.CorrelationID, consumedEvent.CorrelationID)
	}
	if !eventToPublish.Timestamp.Equal(consumedEvent.Timestamp) {
		t.Fatalf("expected timestamp %s, got %s", eventToPublish.Timestamp, consumedEvent.Timestamp)
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

	// Assert prometheus metrics
	if err = assertMetrics(r,
		"amqp_events_publish_succeed",
		"amqp_events_received",
		"amqp_events_ack",
		"amqp_events_processed_duration"); err != nil {
		t.Fatal(err)
	}

	// Dispose outbox
	outboxPublisher.Close()
	db.Close()

	// Dispose amqp connection
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	// Wait for the consumer and outbox loop
	time.Sleep(1000 * time.Millisecond)

	// Stop the notification go routine so goleak does not fail
	exitCh <- true

	// Assert no routine is running at the end
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
