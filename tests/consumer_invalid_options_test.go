package tests

import (
	"testing"

	"github.com/pmorelli92/bunnify/bunnify"
)

func TestConsumerShouldReturnErrorWhenNoHandlersSpecified(t *testing.T) {
	// Setup
	connection := bunnify.NewConnection()
	connection.Start()

	// Exercise
	_, err := connection.NewConsumer("queueName")

	// Assert
	if err == nil {
		t.Fatal(err)
	}
}
