package bunnify

type Notification struct {
	IsError bool
	Message string
}

func sendError(ch chan<- Notification, message string) {
	if ch != nil {
		ch <- Notification{
			IsError: true,
			Message: message,
		}
	}
}

func sendInfo(ch chan<- Notification, message string) {
	if ch != nil {
		ch <- Notification{
			IsError: false,
			Message: message,
		}
	}
}
