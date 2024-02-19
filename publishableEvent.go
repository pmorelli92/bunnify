package bunnify

import (
	"time"

	"github.com/google/uuid"
)

// PublishableEvent represents an event that can be published.
// The Payload field holds the event's payload data, which can be of
// any type that can be marshal to json.
type PublishableEvent struct {
	Metadata
	Payload any `json:"payload"`
}

type eventOptions struct {
	eventID       string
	correlationID string
}

// WithEventID specifies the eventID to be published
// if it is not used a random uuid will be generated.
func WithEventID(eventID string) func(*eventOptions) {
	return func(opt *eventOptions) {
		opt.eventID = eventID
	}
}

// WithCorrelationID specifies the correlationID to be published
// if it is not used a random uuid will be generated.
func WithCorrelationID(correlationID string) func(*eventOptions) {
	return func(opt *eventOptions) {
		opt.correlationID = correlationID
	}
}

// NewPublishableEvent creates an instance of a PublishableEvent.
// In case the ID and correlation ID are not supplied via options random uuid will be generated.
func NewPublishableEvent(payload any, opts ...func(*eventOptions)) PublishableEvent {
	evtOpts := eventOptions{}
	for _, opt := range opts {
		opt(&evtOpts)
	}

	if evtOpts.correlationID == "" {
		evtOpts.correlationID = uuid.NewString()
	}
	if evtOpts.eventID == "" {
		evtOpts.eventID = uuid.NewString()
	}

	return PublishableEvent{
		Metadata: Metadata{
			ID:            evtOpts.eventID,
			CorrelationID: evtOpts.correlationID,
			Timestamp:     time.Now(),
		},
		Payload: payload,
	}
}
