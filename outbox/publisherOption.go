package outbox

import (
	"time"

	"github.com/pmorelli92/bunnify"
)

type publisherOption struct {
	deleteAfterPublish  bool
	loopInterval        time.Duration
	notificationChannel chan<- bunnify.Notification
}

// WithDeleteAfterPublish specifies that a published event from
// the outbox will be deleted instead of marked as published
// as it is not interesting to have the event as a timelog history.
func WithDeleteAfterPublish() func(*publisherOption) {
	return func(opt *publisherOption) {
		opt.deleteAfterPublish = true
	}
}

// WithLoopingInterval specifies the interval on which the
// loop to check the pending to publish events is executed
func WithLoopingInterval(interval time.Duration) func(*publisherOption) {
	return func(opt *publisherOption) {
		opt.loopInterval = interval
	}
}

// WithNoficationChannel specifies a go channel to receive messages
// such as connection established, reconnecting, event published, consumed, etc.
func WithNoficationChannel(notificationCh chan<- bunnify.Notification) func(*publisherOption) {
	return func(opt *publisherOption) {
		opt.notificationChannel = notificationCh
	}
}
