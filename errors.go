package bunnify

import "errors"

var errConnectionClosedByUser = errors.New("connection is already closed by user")
var errMaxReconnectAttemptsReached = errors.New("max reconnect attempts reached")

func isTerminalConnError(err error) bool {
	return errors.Is(err, errConnectionClosedByUser) || errors.Is(err, errMaxReconnectAttemptsReached)
}
