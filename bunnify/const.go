package bunnify

const (
	serverConnectionEstablished = "established connection to server"
	serverConnectionLost        = "lost connection to server, will attempt to reconnect"
	serverConnectionFailed      = "failed to established connection to server, error %s"
	serverClosingConnection     = "closing connection to server"

	channelConnectionEstablished = "established connection to channel"
	channelConnectionLost        = "lost connection to channel, will attempt to reconnect"
	channelConnectionFailed      = "failed to established connection to channel, error %s"
	connectionClosedBySystem     = "connection closed by system, channel will not reconnect"
)
