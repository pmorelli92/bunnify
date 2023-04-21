package tests

import (
	"testing"

	"github.com/pmorelli92/bunnify/bunnify"
)

func TestConsumerShouldReturnErrorWhenNoHandlersSpecified(t *testing.T) {
	// Setup
	connection := bunnify.NewConnection()
	connection.Start()
	consumer := connection.NewConsumer("queueName")

	// Exercise
	err := consumer.Consume()

	// Assert
	if err == nil {
		t.Fatal(err)
	}
}
