package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pmorelli92/bunnify/bunnify"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/goleak"
)

func TestConsumerPublisherTracing(t *testing.T) {
	// Setup tracing
	otel.SetTracerProvider(tracesdk.NewTracerProvider())
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}))

	// Setup amqp
	queueName := uuid.NewString()
	exchangeName := uuid.NewString()
	routingKey := "order.orderCreated"

	type orderCreated struct {
		ID string `json:"id"`
	}

	connection := bunnify.NewConnection()
	connection.Start()

	// Exercise consuming
	var actualTraceID trace.TraceID
	eventHandler := func(ctx context.Context, _ bunnify.ConsumableEvent[orderCreated]) error {
		actualTraceID = trace.SpanFromContext(ctx).SpanContext().TraceID()
		return nil
	}

	consumer := connection.NewConsumer(
		queueName,
		bunnify.WithBindingToExchange(exchangeName),
		bunnify.WithHandler(routingKey, eventHandler))
	if err := consumer.Consume(); err != nil {
		t.Fatal(err)
	}

	// Exercise publishing
	publisher := connection.NewPublisher()
	publishingContext, _ := otel.Tracer("amqp").Start(context.Background(), "publish-test")

	err := publisher.Publish(
		publishingContext,
		exchangeName,
		routingKey,
		bunnify.NewPublishableEvent(orderCreated{
			ID: uuid.NewString(),
		}))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	// Assert
	publishingTraceID := trace.SpanFromContext(publishingContext).SpanContext().TraceID()
	if actualTraceID != publishingTraceID {
		t.Fatalf("expected traceID %s, got %s", publishingTraceID, actualTraceID)
	}

	goleak.VerifyNone(t)
}
