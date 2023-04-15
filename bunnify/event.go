package bunnify

import (
	"encoding/json"
	"time"
)

// Metadata holds the metadata of an event
type Metadata struct {
	ID            string    `json:"id"`
	CorrelationID string    `json:"correlationId"`
	Timestamp     time.Time `json:"timestamp"`
}

// ConsumableEvent[T] represents an event that can be consumed.
// The type parameter T specifies the type of the event's payload.
type ConsumableEvent[T any] struct {
	Metadata
	Payload T
}

// PublishableEvent represents an event that can be published.
// The Payload field holds the event's payload data, which can be of
// any type that can be marshal to json.
type PublishableEvent struct {
	Metadata
	Payload any `json:"payload"`
}

// unmarshalEvent is used internally to unmarshal a PublishableEvent
// this way the payload ends up being a json.RawMessage instead of map[string]interface{}
// so that later the json.RawMessage can be unmarshal to ConsumableEvent[T].Payload
type unmarshalEvent struct {
	Metadata
	Payload json.RawMessage `json:"payload"`
}
