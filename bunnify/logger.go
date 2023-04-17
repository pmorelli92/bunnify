package bunnify

import (
	"os"

	"golang.org/x/exp/slog"
)

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

type Logger interface {
	Info(message string)
	Error(message string)
}

type DefaultLogger struct {
	logger *slog.Logger
}

func NewDefaultLogger() DefaultLogger {
	return DefaultLogger{
		logger: slog.New(slog.NewJSONHandler(os.Stdout)),
	}
}

func (dl DefaultLogger) Info(message string) {
	dl.logger.Info(message)
}

func (dl DefaultLogger) Error(message string) {
	dl.logger.Error(message)
}

type SilentLogger struct {
}

func NewSilentLogger() SilentLogger {
	return SilentLogger{}
}

func (sl SilentLogger) Info(message string) {
}

func (sl SilentLogger) Error(message string) {
}
