package bunnify

import "fmt"

type NotificationProducer string

const (
	NotificationProducerConnection NotificationProducer = "CONNECTION"
	NotificationProducerConsumer   NotificationProducer = "CONSUMER"
	NotificationProducerPublisher  NotificationProducer = "PUBLISHER"
)

type NotificationType string

const (
	NotificationTypeInfo  NotificationType = "INFO"
	NotificationTypeError NotificationType = "ERROR"
)

type Notification struct {
	Message  string
	Type     NotificationType
	Producer NotificationProducer
}

func (n Notification) String() string {
	return fmt.Sprintf("[%s][%s] %s", n.Type, n.Producer, n.Message)
}

func sendError(ch chan<- Notification, producer NotificationProducer, message string) {
	if ch != nil {
		ch <- Notification{
			Message:  message,
			Producer: producer,
			Type:     NotificationTypeError,
		}
	}
}

func sendInfo(ch chan<- Notification, producer NotificationProducer, message string) {
	if ch != nil {
		ch <- Notification{
			Message:  message,
			Producer: producer,
			Type:     NotificationTypeInfo,
		}
	}
}
