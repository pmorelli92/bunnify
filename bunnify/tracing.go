package bunnify

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// inject the span context to amqp table
func injectToHeaders(ctx context.Context) amqp.Table {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	header := amqp.Table{}
	for k, v := range carrier {
		header[k] = v
	}
	return header
}

// extract the amqp table to a span context
func extractToContext(headers amqp.Table) context.Context {
	carrier := propagation.MapCarrier{}
	for k, v := range headers {
		value, ok := v.(string)
		if ok {
			carrier[k] = value
		}
	}

	return otel.GetTextMapPropagator().Extract(context.TODO(), carrier)
}
