package bunnify

import (
	"encoding/json"
	"time"
)

// Metadata holds the metadata of an event.
type Metadata struct {
	ID            string    `json:"id"`
	CorrelationID string    `json:"correlationId"`
	Timestamp     time.Time `json:"timestamp"`
}

// DeliveryInfo holds information of original queue, exchange and routing keys.
type DeliveryInfo struct {
	Queue      string
	Exchange   string
	RoutingKey string
}

// ConsumableEvent[T] represents an event that can be consumed.
// The type parameter T specifies the type of the event's payload.
type ConsumableEvent[T any] struct {
	Metadata
	DeliveryInfo DeliveryInfo
	Payload      T
}

// unmarshalEvent is used internally to unmarshal a PublishableEvent
// this way the payload ends up being a json.RawMessage instead of map[string]interface{}
// so that later the json.RawMessage can be unmarshal to ConsumableEvent[T].Payload.
type unmarshalEvent struct {
	Metadata
	DeliveryInfo DeliveryInfo
	Payload      json.RawMessage `json:"payload"`
}
