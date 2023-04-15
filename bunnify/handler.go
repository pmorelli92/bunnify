package bunnify

import (
	"context"
	"encoding/json"
)

// EventHandler is the type definition for a function that is used to handle events of a specific type.
type EventHandler[T any] func(ctx context.Context, event ConsumableEvent[T]) error

type handlerFor struct {
	routingKey string
	handler    func(ctx context.Context, event unmarshalEvent) error
}

// NewHandlerFor indicates that for given routingKey a specified handler will be called.
// The event to consume must be able to be unmarshal to the specified type T
// which is defined by the ConsumableEvent[T] struct.
func NewHandlerFor[T any](
	routingKey string,
	handler EventHandler[T]) handlerFor {

	return handlerFor{
		routingKey: routingKey,
		handler: func(ctx context.Context, event unmarshalEvent) error {
			consumableEvent := ConsumableEvent[T]{
				Metadata: event.Metadata,
			}

			err := json.Unmarshal(event.Payload, &consumableEvent.Payload)
			if err != nil {
				return err
			}

			return handler(ctx, consumableEvent)
		},
	}
}
