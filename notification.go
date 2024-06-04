package bunnify

import "fmt"

type NotificationSource string

const (
	NotificationSourceConnection NotificationSource = "CONNECTION"
	NotificationSourceConsumer   NotificationSource = "CONSUMER"
	NotificationSourcePublisher  NotificationSource = "PUBLISHER"
)

type NotificationType string

const (
	NotificationTypeInfo  NotificationType = "INFO"
	NotificationTypeError NotificationType = "ERROR"
)

type Notification struct {
	Message string
	Type    NotificationType
	Source  NotificationSource
}

func (n Notification) String() string {
	return fmt.Sprintf("[%s][%s] %s", n.Type, n.Source, n.Message)
}

func notifyConnectionEstablished(ch chan<- Notification) {
	if ch != nil {
		ch <- Notification{
			Type:    NotificationTypeInfo,
			Message: "established connection to server",
			Source:  NotificationSourceConnection,
		}
	}
}

func notifyConnectionLost(ch chan<- Notification) {
	if ch != nil {
		ch <- Notification{
			Type:    NotificationTypeError,
			Message: "lost connection to server, will attempt to reconnect",
			Source:  NotificationSourceConnection,
		}
	}
}

func notifyConnectionFailed(ch chan<- Notification, err error) {
	if ch != nil {
		ch <- Notification{
			Type:    NotificationTypeError,
			Message: fmt.Sprintf("failed to connect to server, error %s", err),
			Source:  NotificationSourceConnection,
		}
	}
}

func notifyClosingConnection(ch chan<- Notification) {
	if ch != nil {
		ch <- Notification{
			Type:    NotificationTypeInfo,
			Message: "closing connection to server",
			Source:  NotificationSourceConnection,
		}
	}
}

func notifyConnectionClosedBySystem(ch chan<- Notification) {
	if ch != nil {
		ch <- Notification{
			Type:    NotificationTypeInfo,
			Message: "connection closed by system, channel will not reconnect",
			Source:  NotificationSourceConnection,
		}
	}
}

func notifyChannelEstablished(ch chan<- Notification, source NotificationSource) {
	if ch != nil {
		ch <- Notification{
			Type:    NotificationTypeInfo,
			Message: "established connection to channel",
			Source:  source,
		}
	}
}

func notifyChannelLost(ch chan<- Notification, source NotificationSource) {
	if ch != nil {
		ch <- Notification{
			Type:    NotificationTypeError,
			Message: "lost connection to channel, will attempt to reconnect",
			Source:  source,
		}
	}
}

func notifyChannelFailed(ch chan<- Notification, source NotificationSource, err error) {
	if ch != nil {
		ch <- Notification{
			Type:    NotificationTypeError,
			Message: fmt.Sprintf("failed to connect to channel, error %s", err),
			Source:  source,
		}
	}
}

func notifyEventHandlerNotFound(ch chan<- Notification, routingKey string) {
	if ch != nil {
		ch <- Notification{
			Type:    NotificationTypeInfo,
			Message: fmt.Sprintf("event handler for %s was not found", routingKey),
			Source:  NotificationSourceConsumer,
		}
	}
}

func notifyEventHandlerSucceed(ch chan<- Notification, routingKey string, took int64) {
	if ch != nil {
		ch <- Notification{
			Type:    NotificationTypeInfo,
			Message: fmt.Sprintf("event handler for %s succeeded, took %d milliseconds", routingKey, took),
			Source:  NotificationSourceConsumer,
		}
	}
}

func notifyEventHandlerFailed(ch chan<- Notification, routingKey string, took int64, err error) {
	if ch != nil {
		ch <- Notification{
			Type:    NotificationTypeError,
			Message: fmt.Sprintf("event handler for %s failed, took %d milliseconds, error: %s", routingKey, took, err),
			Source:  NotificationSourceConsumer,
		}
	}
}
