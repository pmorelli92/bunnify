package outbox

import (
	"fmt"

	"github.com/pmorelli92/bunnify"
)

func notifyEndingLoop(ch chan<- bunnify.Notification) {
	if ch != nil {
		ch <- bunnify.Notification{
			Type:    bunnify.NotificationTypeInfo,
			Message: "the outbox loop has ended",
			Source:  bunnify.NotificationSourcePublisher,
		}
	}
}

func notifyCannotCreateTx(ch chan<- bunnify.Notification) {
	if ch != nil {
		ch <- bunnify.Notification{
			Type:    bunnify.NotificationTypeError,
			Message: "cannot create a transaction for outbox",
			Source:  bunnify.NotificationSourcePublisher,
		}
	}
}

func notifyCannotQueryOutboxTable(ch chan<- bunnify.Notification) {
	if ch != nil {
		ch <- bunnify.Notification{
			Type:    bunnify.NotificationTypeError,
			Message: "cannot query outbox table",
			Source:  bunnify.NotificationSourcePublisher,
		}
	}
}

func notifyCannotPublishAMQP(ch chan<- bunnify.Notification) {
	if ch != nil {
		ch <- bunnify.Notification{
			Type:    bunnify.NotificationTypeError,
			Message: "cannot publish the event to AMQP",
			Source:  bunnify.NotificationSourcePublisher,
		}
	}
}

func notifyCannotMarkOutboxEventsAsPublished(ch chan<- bunnify.Notification) {
	if ch != nil {
		ch <- bunnify.Notification{
			Type:    bunnify.NotificationTypeError,
			Message: "cannot mark outbox events as published",
			Source:  bunnify.NotificationSourcePublisher,
		}
	}
}

func notifyCannotDeleteOutboxEvents(ch chan<- bunnify.Notification) {
	if ch != nil {
		ch <- bunnify.Notification{
			Type:    bunnify.NotificationTypeError,
			Message: "cannot delete outbox events",
			Source:  bunnify.NotificationSourcePublisher,
		}
	}
}

func notifyCannotCommitTransaction(ch chan<- bunnify.Notification) {
	if ch != nil {
		ch <- bunnify.Notification{
			Type:    bunnify.NotificationTypeError,
			Message: "cannot commit transaction",
			Source:  bunnify.NotificationSourcePublisher,
		}
	}
}

func notifyOutboxEventsPublished(ch chan<- bunnify.Notification, quantity int) {
	if ch != nil {
		ch <- bunnify.Notification{
			Type:    bunnify.NotificationTypeInfo,
			Message: fmt.Sprintf("published %d event(s) correctly from outbox", quantity),
			Source:  bunnify.NotificationSourcePublisher,
		}
	}
}
