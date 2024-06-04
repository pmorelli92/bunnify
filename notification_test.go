package bunnify

import (
	"fmt"
	"testing"
)

func TestNotifications(t *testing.T) {
	// Setup
	ch := make(chan Notification, 11)

	// Exercise
	notifyConnectionEstablished(ch)
	notifyConnectionLost(ch)
	notifyConnectionFailed(ch, fmt.Errorf("error"))
	notifyClosingConnection(ch)
	notifyConnectionClosedBySystem(ch)
	notifyChannelEstablished(ch, NotificationSourceConnection)
	notifyChannelLost(ch, NotificationSourceConnection)
	notifyChannelFailed(ch, NotificationSourceConnection, fmt.Errorf("error"))
	notifyEventHandlerSucceed(ch, "routing", 10)
	notifyEventHandlerFailed(ch, "routing", 20, fmt.Errorf("error"))
	notifyEventHandlerNotFound(ch, "routing")

	// Assert
	if (<-ch).Type != NotificationTypeInfo {
		t.Fatal("expected notification type info")
	}
	if (<-ch).Type != NotificationTypeError {
		t.Fatal("expected notification type error")
	}
	if (<-ch).Type != NotificationTypeError {
		t.Fatal("expected notification type error")
	}
	if (<-ch).Type != NotificationTypeInfo {
		t.Fatal("expected notification type info")
	}
	if (<-ch).Type != NotificationTypeInfo {
		t.Fatal("expected notification type info")
	}
	if (<-ch).Type != NotificationTypeInfo {
		t.Fatal("expected notification type info")
	}
	if (<-ch).Type != NotificationTypeError {
		t.Fatal("expected notification type error")
	}
	if (<-ch).Type != NotificationTypeError {
		t.Fatal("expected notification type error")
	}
	if (<-ch).Type != NotificationTypeInfo {
		t.Fatal("expected notification type info")
	}
	if (<-ch).Type != NotificationTypeError {
		t.Fatal("expected notification type error")
	}
	if (<-ch).Type != NotificationTypeError {
		t.Fatal("expected notification type error")
	}
}
