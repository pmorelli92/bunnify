package bunnify

import "errors"

var ErrConnectionClosedByUser = errors.New("connection is already closed by user")
var ErrMaxReconnectAttemptsReached = errors.New("max reconnect attempts reached")

func isTerminalConnError(err error) bool {
	return errors.Is(err, ErrConnectionClosedByUser) || errors.Is(err, ErrMaxReconnectAttemptsReached)
}
