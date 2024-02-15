package bunnify

import (
	"context"
	"encoding/json"
)

// EventHandler is the type definition for a function that is used to handle events of a specific type.
type EventHandler[T any] func(ctx context.Context, event ConsumableEvent[T]) error

// wrappedHandler is internally used to wrap the generic EventHandler
// this is to facilitate adding all the different type of T on the same map
type wrappedHandler func(ctx context.Context, event unmarshalEvent) error

func newWrappedHandler[T any](handler EventHandler[T]) wrappedHandler {
	return func(ctx context.Context, event unmarshalEvent) error {
		consumableEvent := ConsumableEvent[T]{
			Metadata:     event.Metadata,
			DeliveryInfo: event.DeliveryInfo,
		}
		err := json.Unmarshal(event.Payload, &consumableEvent.Payload)
		if err != nil {
			return err
		}
		return handler(ctx, consumableEvent)
	}
}
