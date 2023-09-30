package bunnify

import (
	"context"

	"go.opentelemetry.io/otel"
)

type AMQPHeadersCarrier map[string]any

func (a AMQPHeadersCarrier) Get(key string) string {
	v, ok := a[key]
	if !ok {
		return ""
	}
	return v.(string)
}

func (a AMQPHeadersCarrier) Set(key string, value string) {
	a[key] = value
}

func (a AMQPHeadersCarrier) Keys() []string {
	i := 0
	r := make([]string, len(a))

	for k := range a {
		r[i] = k
		i++
	}

	return r
}

// injects the tracing from the context into the header map
func injectAMQPHeaders(ctx context.Context) map[string]interface{} {
	h := make(AMQPHeadersCarrier)
	otel.GetTextMapPropagator().Inject(ctx, h)
	return h
}

// extracts the tracing from the header and puts it into the context
func extractAMQPHeaders(headers map[string]interface{}) context.Context {
	return otel.GetTextMapPropagator().Extract(context.Background(), AMQPHeadersCarrier(headers))
}
